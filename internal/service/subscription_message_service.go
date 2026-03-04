// Package service provides business logic services.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/repository/cache"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat/client"
)

// SubscriptionMessageService defines the subscription message service interface.
type SubscriptionMessageService interface {
	// SendSubscriptionMessage sends subscription message to user
	SendSubscriptionMessage(ctx context.Context, req *SendSubscriptionMessageRequest) (*SendSubscriptionMessageResponse, error)

	// GetTemplateList gets subscription message template list
	GetTemplateList(ctx context.Context, req *GetTemplateListRequest) (*GetTemplateListResponse, error)
}

// SendSubscriptionMessageRequest represents the service layer request for sending subscription message.
type SendSubscriptionMessageRequest struct {
	AuthorizerAppID  string                 `json:"authorizer_app_id" validate:"required"`
	OpenID           string                 `json:"openid" validate:"required,len=28"`
	TemplateID       string                 `json:"template_id" validate:"required"`
	Data             map[string]interface{} `json:"data" validate:"required,max=20"`
	Page             string                 `json:"page,omitempty"`
	MiniprogramState string                 `json:"miniprogram_state,omitempty"`
	Lang             string                 `json:"lang,omitempty"`
}

// SendSubscriptionMessageResponse represents the service layer response for sending subscription message.
type SendSubscriptionMessageResponse struct {
	MsgID   int64  `json:"msgid,omitempty"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

// GetTemplateListRequest represents the service layer request for getting template list.
type GetTemplateListRequest struct {
	AuthorizerAppID string `json:"authorizer_app_id" validate:"required"`
}

// GetTemplateListResponse represents the service layer response for getting template list.
type GetTemplateListResponse struct {
	Templates []wechat.SubscriptionTemplate `json:"data"`
	ErrCode   int                           `json:"errcode"`
	ErrMsg    string                        `json:"errmsg,omitempty"`
}

// SubscriptionMessageServiceImpl implements SubscriptionMessageService.
type SubscriptionMessageServiceImpl struct {
	tokenService TokenService
	wechatClient client.Client
	cacheRepo    cache.Repository
	logger       *slog.Logger
}

// NewSubscriptionMessageService creates a new subscription message service.
func NewSubscriptionMessageService(
	tokenService TokenService,
	wechatClient client.Client,
	cacheRepo cache.Repository,
	logger *slog.Logger,
) SubscriptionMessageService {
	return &SubscriptionMessageServiceImpl{
		tokenService: tokenService,
		wechatClient: wechatClient,
		cacheRepo:    cacheRepo,
		logger:       logger,
	}
}

// validateRequest validates the send subscription message request.
func (s *SubscriptionMessageServiceImpl) validateRequest(req *SendSubscriptionMessageRequest) error {
	// Validate OpenID length
	if len(req.OpenID) != 28 {
		return fmt.Errorf("invalid openid length: expected 28, got %d", len(req.OpenID))
	}

	// Validate Data field count
	if len(req.Data) > 20 {
		return fmt.Errorf("data fields exceed limit: max 20, got %d", len(req.Data))
	}

	// Validate Data field types and lengths
	for key, value := range req.Data {
		// Check if value is a map with "value" key
		switch v := value.(type) {
		case map[string]interface{}:
			// Subscription message data field format: {"value": "xxx"}
			if val, ok := v["value"].(string); ok {
				if len(val) > 20 {
					return fmt.Errorf("data field '%s' value too long: max 20, got %d", key, len(val))
				}
			} else {
				return fmt.Errorf("data field '%s' must have 'value' as string", key)
			}
		default:
			return fmt.Errorf("data field '%s' must be object with 'value' key", key)
		}
	}

	// Validate page parameter format (if provided)
	if req.Page != "" {
		// Mini program page path format: pages/xxx/xxx
		if !strings.HasPrefix(req.Page, "pages/") {
			return fmt.Errorf("invalid page path format: must start with 'pages/'")
		}
	}

	return nil
}


// isSubscriptionExpiredError checks if the error is a subscription expired error.
func isSubscriptionExpiredError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "code=43101")
}

// isRateLimitError checks if the error is a rate limit error.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "code=45009")
}

// handleError handles and categorizes WeChat API errors.
func (s *SubscriptionMessageServiceImpl) handleError(err error, operation string) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Token expired error - already handled in calling layer with retry
	if strings.Contains(errMsg, "code=40001") || strings.Contains(errMsg, "code=42001") {
		return fmt.Errorf("token expired: %w", err)
	}

	// Subscription status error
	if strings.Contains(errMsg, "code=43101") {
		return fmt.Errorf("subscription expired or rejected: %w", err)
	}

	// Template not found
	if strings.Contains(errMsg, "code=40037") {
		return fmt.Errorf("template not found: %w", err)
	}

	// Rate limit
	if strings.Contains(errMsg, "code=45009") {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	// OpenID invalid
	if strings.Contains(errMsg, "code=40003") {
		return fmt.Errorf("invalid openid: %w", err)
	}

	// Data field too long
	if strings.Contains(errMsg, "code=47003") {
		return fmt.Errorf("data field too long: %w", err)
	}

	// Other errors
	return fmt.Errorf("%s failed: %w", operation, err)
}

// SendSubscriptionMessage sends subscription message to user.
func (s *SubscriptionMessageServiceImpl) SendSubscriptionMessage(
	ctx context.Context,
	req *SendSubscriptionMessageRequest,
) (*SendSubscriptionMessageResponse, error) {
	// 1. Ensure request ID exists
	ctx, requestID := EnsureRequestID(ctx)
	serviceStart := time.Now()

	s.logger.Info("[SendSubscriptionMessage] started",
		slog.String("request_id", requestID),
		slog.String("appid", req.AuthorizerAppID),
		slog.String("openid", req.OpenID),
		slog.String("template_id", req.TemplateID),
	)

	// 2. Parameter validation
	if err := s.validateRequest(req); err != nil {
		s.logger.Error("[SendSubscriptionMessage] validation failed",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 3. Get access token
	tokenStart := time.Now()
	token, err := s.tokenService.GetAuthorizerToken(ctx, req.AuthorizerAppID)
	tokenDuration := time.Since(tokenStart)

	if err != nil {
		s.logger.Error("[SendSubscriptionMessage] failed to get token",
			slog.String("request_id", requestID),
			slog.Duration("token_duration", tokenDuration),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// 4. Construct WeChat API request
	wechatReq := &wechat.SendSubscriptionMessageRequest{
		ToUser:           req.OpenID,
		TemplateID:       req.TemplateID,
		Page:             req.Page,
		Data:             req.Data,
		MiniprogramState: req.MiniprogramState,
		Lang:             req.Lang,
	}

	// 5. Call WeChat API
	apiStart := time.Now()
	resp, err := s.wechatClient.SendSubscriptionMessage(ctx, token, wechatReq)
	apiDuration := time.Since(apiStart)

	// 6. Handle token expiration error (auto retry)
	if err != nil && isTokenExpiredError(err) {
		s.logger.Warn("[SendSubscriptionMessage] token expired, retrying",
			slog.String("request_id", requestID),
			slog.Duration("api_duration", apiDuration),
		)

		// Refresh token
		refreshStart := time.Now()
		token, err = s.tokenService.InvalidateAndRefreshToken(ctx, req.AuthorizerAppID)
		refreshDuration := time.Since(refreshStart)

		if err != nil {
			s.logger.Error("[SendSubscriptionMessage] token refresh failed",
				slog.String("request_id", requestID),
				slog.Duration("refresh_duration", refreshDuration),
				slog.String("error", err.Error()),
			)
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		// Retry API call
		retryStart := time.Now()
		resp, err = s.wechatClient.SendSubscriptionMessage(ctx, token, wechatReq)
		apiDuration = time.Since(retryStart)
	}

	// 7. Handle final result
	totalDuration := time.Since(serviceStart)

	if err != nil {
		s.logger.Error("[SendSubscriptionMessage] failed",
			slog.String("request_id", requestID),
			slog.Duration("api_duration", apiDuration),
			slog.Duration("total_duration", totalDuration),
			slog.String("error", err.Error()),
		)
		return nil, s.handleError(err, "send subscription message")
	}

	s.logger.Info("[SendSubscriptionMessage] completed",
		slog.String("request_id", requestID),
		slog.Int64("msgid", resp.MsgID),
		slog.Duration("token_duration", tokenDuration),
		slog.Duration("api_duration", apiDuration),
		slog.Duration("total_duration", totalDuration),
	)

	return &SendSubscriptionMessageResponse{
		MsgID:   resp.MsgID,
		ErrCode: resp.ErrCode,
		ErrMsg:  resp.ErrMsg,
	}, nil
}

// GetTemplateList gets subscription message template list.
func (s *SubscriptionMessageServiceImpl) GetTemplateList(
	ctx context.Context,
	req *GetTemplateListRequest,
) (*GetTemplateListResponse, error) {
	ctx, requestID := EnsureRequestID(ctx)
	serviceStart := time.Now()

	s.logger.Info("[GetTemplateList] started",
		slog.String("request_id", requestID),
		slog.String("appid", req.AuthorizerAppID),
	)

	// 1. Check cache
	cacheKey := cache.FormatTemplateListKey(req.AuthorizerAppID)
	cacheStart := time.Now()

	var cachedTemplates []wechat.SubscriptionTemplate
	err := s.cacheRepo.Get(ctx, cacheKey, &cachedTemplates)
	cacheDuration := time.Since(cacheStart)

	if err == nil && len(cachedTemplates) > 0 {
		s.logger.Debug("[GetTemplateList] cache hit",
			slog.String("request_id", requestID),
			slog.Duration("cache_duration", cacheDuration),
			slog.Int("template_count", len(cachedTemplates)),
		)

		return &GetTemplateListResponse{
			Templates: cachedTemplates,
			ErrCode:   0,
		}, nil
	}

	// 2. Get access token
	tokenStart := time.Now()
	token, err := s.tokenService.GetAuthorizerToken(ctx, req.AuthorizerAppID)
	tokenDuration := time.Since(tokenStart)

	if err != nil {
		s.logger.Error("[GetTemplateList] failed to get token",
			slog.String("request_id", requestID),
			slog.Duration("token_duration", tokenDuration),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// 3. Call WeChat API
	resp, err := s.wechatClient.GetSubscriptionTemplateList(ctx, token)

	// 4. Handle token expiration (retry)
	if err != nil && isTokenExpiredError(err) {
		s.logger.Warn("[GetTemplateList] token expired, retrying",
			slog.String("request_id", requestID),
		)

		token, err = s.tokenService.InvalidateAndRefreshToken(ctx, req.AuthorizerAppID)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		resp, err = s.wechatClient.GetSubscriptionTemplateList(ctx, token)
	}

	if err != nil {
		s.logger.Error("[GetTemplateList] failed",
			slog.String("request_id", requestID),
			slog.Duration("total_duration", time.Since(serviceStart)),
			slog.String("error", err.Error()),
		)
		return nil, s.handleError(err, "get template list")
	}

	// 5. Cache result (30 minutes)
	if len(resp.Data) > 0 {
		cacheErr := s.cacheRepo.Set(ctx, cacheKey, resp.Data, 30*time.Minute)
		if cacheErr != nil {
			s.logger.Warn("[GetTemplateList] cache write failed",
				slog.String("request_id", requestID),
				slog.String("error", cacheErr.Error()),
			)
		}
	}

	s.logger.Info("[GetTemplateList] completed",
		slog.String("request_id", requestID),
		slog.Int("template_count", len(resp.Data)),
		slog.Duration("total_duration", time.Since(serviceStart)),
	)

	return &GetTemplateListResponse{
		Templates: resp.Data,
		ErrCode:   resp.ErrCode,
		ErrMsg:    resp.ErrMsg,
	}, nil
}
