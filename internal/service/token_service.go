// Package service provides business logic services.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/singleflight"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/config"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/repository/cache"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat/client"
)

// ProactiveRefreshThreshold is the time before expiration to trigger proactive refresh
const ProactiveRefreshThreshold = 10 * time.Minute

// TokenService defines the token management service interface.
type TokenService interface {
	// GetComponentToken returns the component_access_token
	GetComponentToken(ctx context.Context) (string, error)

	// GetAuthorizerToken returns the authorizer_access_token for the given appid
	GetAuthorizerToken(ctx context.Context, authorizerAppID string) (string, error)

	// InvalidateAndRefreshToken invalidates cached token and fetches a new one
	InvalidateAndRefreshToken(ctx context.Context, authorizerAppID string) (string, error)
}

// TokenServiceImpl implements TokenService.
type TokenServiceImpl struct {
	config       *config.WeChatConfig
	cacheRepo    cache.Repository
	wechatClient client.Client
	sfGroup      singleflight.Group
	logger       *slog.Logger
}

// NewTokenService creates a new TokenService.
func NewTokenService(
	cfg *config.WeChatConfig,
	cacheRepo cache.Repository,
	wechatClient client.Client,
	logger *slog.Logger,
) *TokenServiceImpl {
	return &TokenServiceImpl{
		config:       cfg,
		cacheRepo:    cacheRepo,
		wechatClient: wechatClient,
		logger:       logger,
	}
}

// GetComponentToken returns the component_access_token.
// It first checks the cache, then fetches from WeChat API if not found or expired.
func (s *TokenServiceImpl) GetComponentToken(ctx context.Context) (string, error) {
	componentAppID := s.config.Component.AppID

	// Check cache first
	token, err := s.cacheRepo.GetComponentToken(ctx, componentAppID)
	if err != nil {
		s.logger.Warn("failed to get component token from cache, will fetch from API",
			slog.String("error", err.Error()),
		)
	}

	if token != "" {
		// Check if proactive refresh is needed
		key := cache.FormatComponentTokenKey(componentAppID)
		ttl, err := s.cacheRepo.GetTokenTTL(ctx, key)
		if err == nil && ttl > 0 && ttl < ProactiveRefreshThreshold {
			// Trigger async refresh
			go s.refreshComponentToken(context.Background())
		}
		return token, nil
	}

	// Use singleflight to prevent duplicate refresh
	result, err, _ := s.sfGroup.Do("component_token:"+componentAppID, func() (interface{}, error) {
		return s.fetchAndCacheComponentToken(ctx)
	})

	if err != nil {
		s.logger.Error("failed to get component token",
			slog.String("component_appid", componentAppID),
			slog.String("error", err.Error()),
		)
		return "", err
	}

	return result.(string), nil
}

// GetAuthorizerToken returns the authorizer_access_token for the given appid.
func (s *TokenServiceImpl) GetAuthorizerToken(ctx context.Context, authorizerAppID string) (string, error) {
	// Check cache first
	token, err := s.cacheRepo.GetAuthorizerToken(ctx, authorizerAppID)
	if err != nil {
		s.logger.Warn("failed to get authorizer token from cache, will fetch from API",
			slog.String("authorizer_appid", authorizerAppID),
			slog.String("error", err.Error()),
		)
	}

	if token != "" {
		// Check if proactive refresh is needed
		key := cache.FormatAuthorizerTokenKey(authorizerAppID)
		ttl, err := s.cacheRepo.GetTokenTTL(ctx, key)
		if err == nil && ttl > 0 && ttl < ProactiveRefreshThreshold {
			// Trigger async refresh
			go s.refreshAuthorizerToken(context.Background(), authorizerAppID)
		}
		return token, nil
	}

	// Use singleflight to prevent duplicate refresh
	result, err, _ := s.sfGroup.Do("authorizer_token:"+authorizerAppID, func() (interface{}, error) {
		// Check if simple mode is enabled
		if s.config.IsSimpleMode() {
			return s.fetchAndCacheSimpleModeToken(ctx, authorizerAppID)
		}
		return s.fetchAndCacheAuthorizerToken(ctx, authorizerAppID)
	})

	if err != nil {
		s.logger.Error("failed to get authorizer token",
			slog.String("authorizer_appid", authorizerAppID),
			slog.String("error", err.Error()),
		)
		return "", err
	}

	return result.(string), nil
}

// fetchAndCacheComponentToken fetches component token from WeChat API and caches it.
func (s *TokenServiceImpl) fetchAndCacheComponentToken(ctx context.Context) (string, error) {
	req := &wechat.ComponentTokenRequest{
		ComponentAppID:        s.config.Component.AppID,
		ComponentAppSecret:    s.config.Component.AppSecret,
		ComponentVerifyTicket: s.config.Component.VerifyTicket,
	}

	resp, err := s.wechatClient.GetComponentAccessToken(ctx, req)
	if err != nil {
		s.logger.Error("ALERT: failed to fetch component token from WeChat API",
			slog.String("component_appid", s.config.Component.AppID),
			slog.String("error", err.Error()),
		)
		return "", fmt.Errorf("failed to fetch component token: %w", err)
	}

	// Cache the token
	if err := s.cacheRepo.SetComponentToken(ctx, s.config.Component.AppID, resp.ComponentAccessToken, resp.ExpiresIn); err != nil {
		s.logger.Warn("failed to cache component token",
			slog.String("error", err.Error()),
		)
		// Don't return error, token is still valid
	}

	s.logger.Info("component token refreshed successfully",
		slog.String("component_appid", s.config.Component.AppID),
		slog.Int("expires_in", resp.ExpiresIn),
	)

	return resp.ComponentAccessToken, nil
}

// fetchAndCacheAuthorizerToken fetches authorizer token from WeChat API and caches it.
func (s *TokenServiceImpl) fetchAndCacheAuthorizerToken(ctx context.Context, authorizerAppID string) (string, error) {
	// Get authorizer config
	authConfig, found := s.config.GetAuthorizerByAppID(authorizerAppID)
	if !found {
		return "", fmt.Errorf("authorizer not found: %s", authorizerAppID)
	}

	// Get component token first
	componentToken, err := s.GetComponentToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get component token: %w", err)
	}

	req := &wechat.RefreshAuthorizerTokenRequest{
		ComponentAppID:         s.config.Component.AppID,
		AuthorizerAppID:        authorizerAppID,
		AuthorizerRefreshToken: authConfig.RefreshToken,
	}

	resp, err := s.wechatClient.RefreshAuthorizerToken(ctx, componentToken, req)
	if err != nil {
		s.logger.Error("ALERT: failed to refresh authorizer token from WeChat API",
			slog.String("authorizer_appid", authorizerAppID),
			slog.String("error", err.Error()),
		)
		return "", fmt.Errorf("failed to refresh authorizer token: %w", err)
	}

	// Cache the token
	if err := s.cacheRepo.SetAuthorizerToken(ctx, authorizerAppID, resp.AuthorizerAccessToken, resp.ExpiresIn); err != nil {
		s.logger.Warn("failed to cache authorizer token",
			slog.String("authorizer_appid", authorizerAppID),
			slog.String("error", err.Error()),
		)
		// Don't return error, token is still valid
	}

	s.logger.Info("authorizer token refreshed successfully",
		slog.String("authorizer_appid", authorizerAppID),
		slog.Int("expires_in", resp.ExpiresIn),
	)

	return resp.AuthorizerAccessToken, nil
}

// refreshComponentToken refreshes component token asynchronously.
func (s *TokenServiceImpl) refreshComponentToken(ctx context.Context) {
	_, err, _ := s.sfGroup.Do("component_token:"+s.config.Component.AppID, func() (interface{}, error) {
		return s.fetchAndCacheComponentToken(ctx)
	})
	if err != nil {
		s.logger.Error("ALERT: proactive component token refresh failed",
			slog.String("error", err.Error()),
		)
	}
}

// refreshAuthorizerToken refreshes authorizer token asynchronously.
func (s *TokenServiceImpl) refreshAuthorizerToken(ctx context.Context, authorizerAppID string) {
	_, err, _ := s.sfGroup.Do("authorizer_token:"+authorizerAppID, func() (interface{}, error) {
		if s.config.IsSimpleMode() {
			return s.fetchAndCacheSimpleModeToken(ctx, authorizerAppID)
		}
		return s.fetchAndCacheAuthorizerToken(ctx, authorizerAppID)
	})
	if err != nil {
		s.logger.Error("ALERT: proactive authorizer token refresh failed",
			slog.String("authorizer_appid", authorizerAppID),
			slog.String("error", err.Error()),
		)
	}
}

// fetchAndCacheSimpleModeToken fetches access_token directly using appid/appsecret (simple mode).
func (s *TokenServiceImpl) fetchAndCacheSimpleModeToken(ctx context.Context, appID string) (string, error) {
	// Get simple account config
	account, found := s.config.GetSimpleAccountByAppID(appID)
	if !found {
		return "", fmt.Errorf("account not found in simple_mode.accounts: %s", appID)
	}

	// Fetch access_token from WeChat API
	resp, err := s.wechatClient.GetAccessToken(ctx, account.AppID, account.AppSecret)
	if err != nil {
		s.logger.Error("ALERT: failed to fetch access_token from WeChat API (simple mode)",
			slog.String("appid", appID),
			slog.String("error", err.Error()),
		)
		return "", fmt.Errorf("failed to fetch access_token: %w", err)
	}

	// Cache the token
	if err := s.cacheRepo.SetAuthorizerToken(ctx, appID, resp.AccessToken, resp.ExpiresIn); err != nil {
		s.logger.Warn("failed to cache access_token",
			slog.String("appid", appID),
			slog.String("error", err.Error()),
		)
	}

	s.logger.Info("access_token refreshed successfully (simple mode)",
		slog.String("appid", appID),
		slog.Int("expires_in", resp.ExpiresIn),
	)

	return resp.AccessToken, nil
}

// InvalidateAndRefreshToken invalidates the cached token and fetches a new one.
// This is used when the API returns token expired error.
func (s *TokenServiceImpl) InvalidateAndRefreshToken(ctx context.Context, authorizerAppID string) (string, error) {
	// Delete cached token first
	key := cache.FormatAuthorizerTokenKey(authorizerAppID)
	if err := s.cacheRepo.DeleteToken(ctx, key); err != nil {
		s.logger.Warn("failed to delete cached token",
			slog.String("authorizer_appid", authorizerAppID),
			slog.String("error", err.Error()),
		)
	}

	s.logger.Info("token invalidated, fetching new token",
		slog.String("authorizer_appid", authorizerAppID),
	)

	// Fetch new token
	if s.config.IsSimpleMode() {
		return s.fetchAndCacheSimpleModeToken(ctx, authorizerAppID)
	}
	return s.fetchAndCacheAuthorizerToken(ctx, authorizerAppID)
}
