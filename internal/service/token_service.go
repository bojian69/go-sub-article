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
func (s *TokenServiceImpl) GetComponentToken(ctx context.Context) (string, error) {
	requestID := GetRequestID(ctx)
	componentAppID := s.config.Component.AppID
	start := time.Now()

	// Check cache first
	cacheStart := time.Now()
	token, err := s.cacheRepo.GetComponentToken(ctx, componentAppID)
	cacheDuration := time.Since(cacheStart)

	if err != nil {
		s.logger.Warn("[TokenService] cache read failed",
			slog.String("request_id", requestID),
			slog.String("type", "component"),
			slog.String("appid", componentAppID),
			slog.Duration("cache_duration", cacheDuration),
			slog.String("error", err.Error()),
		)
	}

	if token != "" {
		s.logger.Debug("[TokenService] cache hit",
			slog.String("request_id", requestID),
			slog.String("type", "component"),
			slog.String("appid", componentAppID),
			slog.Duration("cache_duration", cacheDuration),
		)

		// Check if proactive refresh is needed
		key := cache.FormatComponentTokenKey(componentAppID)
		ttl, err := s.cacheRepo.GetTokenTTL(ctx, key)
		if err == nil && ttl > 0 && ttl < ProactiveRefreshThreshold {
			s.logger.Info("[TokenService] proactive refresh triggered",
				slog.String("request_id", requestID),
				slog.String("type", "component"),
				slog.Duration("ttl_remaining", ttl),
			)
			go s.refreshComponentToken(context.Background())
		}
		return token, nil
	}

	s.logger.Debug("[TokenService] cache miss, fetching from API",
		slog.String("request_id", requestID),
		slog.String("type", "component"),
		slog.String("appid", componentAppID),
		slog.Duration("cache_duration", cacheDuration),
	)

	// Use singleflight to prevent duplicate refresh
	result, err, shared := s.sfGroup.Do("component_token:"+componentAppID, func() (interface{}, error) {
		return s.fetchAndCacheComponentToken(ctx)
	})

	totalDuration := time.Since(start)
	if err != nil {
		s.logger.Error("[TokenService] failed to get component token",
			slog.String("request_id", requestID),
			slog.String("appid", componentAppID),
			slog.Bool("shared", shared),
			slog.Duration("total_duration", totalDuration),
			slog.String("error", err.Error()),
		)
		return "", err
	}

	s.logger.Debug("[TokenService] component token acquired",
		slog.String("request_id", requestID),
		slog.String("appid", componentAppID),
		slog.Bool("shared", shared),
		slog.Duration("total_duration", totalDuration),
	)

	return result.(string), nil
}

// GetAuthorizerToken returns the authorizer_access_token for the given appid.
func (s *TokenServiceImpl) GetAuthorizerToken(ctx context.Context, authorizerAppID string) (string, error) {
	requestID := GetRequestID(ctx)
	start := time.Now()

	// Check cache first
	cacheStart := time.Now()
	token, err := s.cacheRepo.GetAuthorizerToken(ctx, authorizerAppID)
	cacheDuration := time.Since(cacheStart)

	if err != nil {
		s.logger.Warn("[TokenService] cache read failed",
			slog.String("request_id", requestID),
			slog.String("type", "authorizer"),
			slog.String("appid", authorizerAppID),
			slog.Duration("cache_duration", cacheDuration),
			slog.String("error", err.Error()),
		)
	}

	if token != "" {
		s.logger.Debug("[TokenService] cache hit",
			slog.String("request_id", requestID),
			slog.String("type", "authorizer"),
			slog.String("appid", authorizerAppID),
			slog.Duration("cache_duration", cacheDuration),
		)

		// Check if proactive refresh is needed
		key := cache.FormatAuthorizerTokenKey(authorizerAppID)
		ttl, err := s.cacheRepo.GetTokenTTL(ctx, key)
		if err == nil && ttl > 0 && ttl < ProactiveRefreshThreshold {
			s.logger.Info("[TokenService] proactive refresh triggered",
				slog.String("request_id", requestID),
				slog.String("type", "authorizer"),
				slog.String("appid", authorizerAppID),
				slog.Duration("ttl_remaining", ttl),
			)
			go s.refreshAuthorizerToken(context.Background(), authorizerAppID)
		}
		return token, nil
	}

	s.logger.Debug("[TokenService] cache miss, fetching from API",
		slog.String("request_id", requestID),
		slog.String("type", "authorizer"),
		slog.String("appid", authorizerAppID),
		slog.Duration("cache_duration", cacheDuration),
	)

	// Use singleflight to prevent duplicate refresh
	result, err, shared := s.sfGroup.Do("authorizer_token:"+authorizerAppID, func() (interface{}, error) {
		if s.config.IsSimpleMode() {
			return s.fetchAndCacheSimpleModeToken(ctx, authorizerAppID)
		}
		return s.fetchAndCacheAuthorizerToken(ctx, authorizerAppID)
	})

	totalDuration := time.Since(start)
	if err != nil {
		s.logger.Error("[TokenService] failed to get authorizer token",
			slog.String("request_id", requestID),
			slog.String("appid", authorizerAppID),
			slog.Bool("shared", shared),
			slog.Duration("total_duration", totalDuration),
			slog.String("error", err.Error()),
		)
		return "", err
	}

	s.logger.Debug("[TokenService] authorizer token acquired",
		slog.String("request_id", requestID),
		slog.String("appid", authorizerAppID),
		slog.Bool("shared", shared),
		slog.Duration("total_duration", totalDuration),
	)

	return result.(string), nil
}

// fetchAndCacheComponentToken fetches component token from WeChat API and caches it.
func (s *TokenServiceImpl) fetchAndCacheComponentToken(ctx context.Context) (string, error) {
	requestID := GetRequestID(ctx)
	start := time.Now()

	req := &wechat.ComponentTokenRequest{
		ComponentAppID:        s.config.Component.AppID,
		ComponentAppSecret:    s.config.Component.AppSecret,
		ComponentVerifyTicket: s.config.Component.VerifyTicket,
	}

	apiStart := time.Now()
	resp, err := s.wechatClient.GetComponentAccessToken(ctx, req)
	apiDuration := time.Since(apiStart)

	if err != nil {
		s.logger.Error("[TokenService] WeChat API call failed",
			slog.String("request_id", requestID),
			slog.String("api", "GetComponentAccessToken"),
			slog.String("appid", s.config.Component.AppID),
			slog.Duration("api_duration", apiDuration),
			slog.String("error", err.Error()),
		)
		return "", fmt.Errorf("failed to fetch component token: %w", err)
	}

	// Cache the token
	cacheStart := time.Now()
	cacheErr := s.cacheRepo.SetComponentToken(ctx, s.config.Component.AppID, resp.ComponentAccessToken, resp.ExpiresIn)
	cacheDuration := time.Since(cacheStart)

	if cacheErr != nil {
		s.logger.Warn("[TokenService] cache write failed",
			slog.String("request_id", requestID),
			slog.String("type", "component"),
			slog.Duration("cache_duration", cacheDuration),
			slog.String("error", cacheErr.Error()),
		)
	}

	totalDuration := time.Since(start)
	s.logger.Info("[TokenService] component token refreshed",
		slog.String("request_id", requestID),
		slog.String("appid", s.config.Component.AppID),
		slog.Int("expires_in", resp.ExpiresIn),
		slog.Duration("api_duration", apiDuration),
		slog.Duration("cache_duration", cacheDuration),
		slog.Duration("total_duration", totalDuration),
	)

	return resp.ComponentAccessToken, nil
}

// fetchAndCacheAuthorizerToken fetches authorizer token from WeChat API and caches it.
func (s *TokenServiceImpl) fetchAndCacheAuthorizerToken(ctx context.Context, authorizerAppID string) (string, error) {
	requestID := GetRequestID(ctx)
	start := time.Now()

	// Get authorizer config
	authConfig, found := s.config.GetAuthorizerByAppID(authorizerAppID)
	if !found {
		return "", fmt.Errorf("authorizer not found: %s", authorizerAppID)
	}

	// Get component token first
	componentStart := time.Now()
	componentToken, err := s.GetComponentToken(ctx)
	componentDuration := time.Since(componentStart)

	if err != nil {
		s.logger.Error("[TokenService] failed to get component token for authorizer refresh",
			slog.String("request_id", requestID),
			slog.String("appid", authorizerAppID),
			slog.Duration("component_duration", componentDuration),
			slog.String("error", err.Error()),
		)
		return "", fmt.Errorf("failed to get component token: %w", err)
	}

	req := &wechat.RefreshAuthorizerTokenRequest{
		ComponentAppID:         s.config.Component.AppID,
		AuthorizerAppID:        authorizerAppID,
		AuthorizerRefreshToken: authConfig.RefreshToken,
	}

	apiStart := time.Now()
	resp, err := s.wechatClient.RefreshAuthorizerToken(ctx, componentToken, req)
	apiDuration := time.Since(apiStart)

	if err != nil {
		s.logger.Error("[TokenService] WeChat API call failed",
			slog.String("request_id", requestID),
			slog.String("api", "RefreshAuthorizerToken"),
			slog.String("appid", authorizerAppID),
			slog.Duration("api_duration", apiDuration),
			slog.String("error", err.Error()),
		)
		return "", fmt.Errorf("failed to refresh authorizer token: %w", err)
	}

	// Cache the token
	cacheStart := time.Now()
	cacheErr := s.cacheRepo.SetAuthorizerToken(ctx, authorizerAppID, resp.AuthorizerAccessToken, resp.ExpiresIn)
	cacheDuration := time.Since(cacheStart)

	if cacheErr != nil {
		s.logger.Warn("[TokenService] cache write failed",
			slog.String("request_id", requestID),
			slog.String("type", "authorizer"),
			slog.String("appid", authorizerAppID),
			slog.Duration("cache_duration", cacheDuration),
			slog.String("error", cacheErr.Error()),
		)
	}

	totalDuration := time.Since(start)
	s.logger.Info("[TokenService] authorizer token refreshed",
		slog.String("request_id", requestID),
		slog.String("appid", authorizerAppID),
		slog.Int("expires_in", resp.ExpiresIn),
		slog.Duration("component_duration", componentDuration),
		slog.Duration("api_duration", apiDuration),
		slog.Duration("cache_duration", cacheDuration),
		slog.Duration("total_duration", totalDuration),
	)

	return resp.AuthorizerAccessToken, nil
}

// fetchAndCacheSimpleModeToken fetches access_token directly using appid/appsecret (simple mode).
func (s *TokenServiceImpl) fetchAndCacheSimpleModeToken(ctx context.Context, appID string) (string, error) {
	requestID := GetRequestID(ctx)
	start := time.Now()

	// Get simple account config
	account, found := s.config.GetSimpleAccountByAppID(appID)
	if !found {
		return "", fmt.Errorf("account not found in simple_mode.accounts: %s", appID)
	}

	// Fetch access_token from WeChat API
	apiStart := time.Now()
	resp, err := s.wechatClient.GetAccessToken(ctx, account.AppID, account.AppSecret)
	apiDuration := time.Since(apiStart)

	if err != nil {
		s.logger.Error("[TokenService] WeChat API call failed (simple mode)",
			slog.String("request_id", requestID),
			slog.String("api", "GetAccessToken"),
			slog.String("appid", appID),
			slog.Duration("api_duration", apiDuration),
			slog.String("error", err.Error()),
		)
		return "", fmt.Errorf("failed to fetch access_token: %w", err)
	}

	// Cache the token
	cacheStart := time.Now()
	cacheErr := s.cacheRepo.SetAuthorizerToken(ctx, appID, resp.AccessToken, resp.ExpiresIn)
	cacheDuration := time.Since(cacheStart)

	if cacheErr != nil {
		s.logger.Warn("[TokenService] cache write failed",
			slog.String("request_id", requestID),
			slog.String("type", "simple_mode"),
			slog.String("appid", appID),
			slog.Duration("cache_duration", cacheDuration),
			slog.String("error", cacheErr.Error()),
		)
	}

	totalDuration := time.Since(start)
	s.logger.Info("[TokenService] access_token refreshed (simple mode)",
		slog.String("request_id", requestID),
		slog.String("appid", appID),
		slog.Int("expires_in", resp.ExpiresIn),
		slog.Duration("api_duration", apiDuration),
		slog.Duration("cache_duration", cacheDuration),
		slog.Duration("total_duration", totalDuration),
	)

	return resp.AccessToken, nil
}

// refreshComponentToken refreshes component token asynchronously.
func (s *TokenServiceImpl) refreshComponentToken(ctx context.Context) {
	_, err, _ := s.sfGroup.Do("component_token:"+s.config.Component.AppID, func() (interface{}, error) {
		return s.fetchAndCacheComponentToken(ctx)
	})
	if err != nil {
		s.logger.Error("[TokenService] proactive refresh failed",
			slog.String("type", "component"),
			slog.String("appid", s.config.Component.AppID),
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
		s.logger.Error("[TokenService] proactive refresh failed",
			slog.String("type", "authorizer"),
			slog.String("appid", authorizerAppID),
			slog.String("error", err.Error()),
		)
	}
}

// InvalidateAndRefreshToken invalidates the cached token and fetches a new one.
func (s *TokenServiceImpl) InvalidateAndRefreshToken(ctx context.Context, authorizerAppID string) (string, error) {
	requestID := GetRequestID(ctx)
	start := time.Now()

	// Delete cached token first
	key := cache.FormatAuthorizerTokenKey(authorizerAppID)
	deleteStart := time.Now()
	deleteErr := s.cacheRepo.DeleteToken(ctx, key)
	deleteDuration := time.Since(deleteStart)

	if deleteErr != nil {
		s.logger.Warn("[TokenService] cache delete failed",
			slog.String("request_id", requestID),
			slog.String("appid", authorizerAppID),
			slog.Duration("delete_duration", deleteDuration),
			slog.String("error", deleteErr.Error()),
		)
	} else {
		s.logger.Info("[TokenService] token invalidated",
			slog.String("request_id", requestID),
			slog.String("appid", authorizerAppID),
			slog.Duration("delete_duration", deleteDuration),
		)
	}

	// Fetch new token
	var token string
	var err error
	if s.config.IsSimpleMode() {
		token, err = s.fetchAndCacheSimpleModeToken(ctx, authorizerAppID)
	} else {
		token, err = s.fetchAndCacheAuthorizerToken(ctx, authorizerAppID)
	}

	totalDuration := time.Since(start)
	if err != nil {
		s.logger.Error("[TokenService] invalidate and refresh failed",
			slog.String("request_id", requestID),
			slog.String("appid", authorizerAppID),
			slog.Duration("total_duration", totalDuration),
			slog.String("error", err.Error()),
		)
	} else {
		s.logger.Info("[TokenService] invalidate and refresh completed",
			slog.String("request_id", requestID),
			slog.String("appid", authorizerAppID),
			slog.Duration("total_duration", totalDuration),
		)
	}

	return token, err
}
