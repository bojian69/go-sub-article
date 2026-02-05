package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat/client"
)

// ArticleService defines the article service interface.
type ArticleService interface {
	// BatchGetPublishedArticles gets published articles list
	BatchGetPublishedArticles(ctx context.Context, req *BatchGetArticlesRequest) (*BatchGetArticlesResponse, error)

	// GetPublishedArticle gets article details
	GetPublishedArticle(ctx context.Context, req *GetArticleRequest) (*GetArticleResponse, error)
}

// BatchGetArticlesRequest represents the request to get articles list.
type BatchGetArticlesRequest struct {
	AuthorizerAppID string `json:"authorizer_app_id" validate:"required"`
	Offset          int    `json:"offset" validate:"gte=0"`
	Count           int    `json:"count" validate:"gte=1,lte=20"`
	NoContent       int    `json:"no_content" validate:"oneof=0 1"`
}

// BatchGetArticlesResponse represents the response of articles list.
type BatchGetArticlesResponse struct {
	TotalCount int                       `json:"total_count"`
	ItemCount  int                       `json:"item_count"`
	Item       []wechat.PublishedArticle `json:"item"`
}

// GetArticleRequest represents the request to get article details.
type GetArticleRequest struct {
	AuthorizerAppID string `json:"authorizer_app_id" validate:"required"`
	ArticleID       string `json:"article_id" validate:"required"`
}

// GetArticleResponse represents the response of article details.
type GetArticleResponse struct {
	NewsItem []wechat.NewsItem `json:"news_item"`
}

// ArticleServiceImpl implements ArticleService.
type ArticleServiceImpl struct {
	tokenService TokenService
	wechatClient client.Client
	logger       *slog.Logger
}

// NewArticleService creates a new ArticleService.
func NewArticleService(
	tokenService TokenService,
	wechatClient client.Client,
	logger *slog.Logger,
) *ArticleServiceImpl {
	return &ArticleServiceImpl{
		tokenService: tokenService,
		wechatClient: wechatClient,
		logger:       logger,
	}
}

// BatchGetPublishedArticles gets published articles list.
func (s *ArticleServiceImpl) BatchGetPublishedArticles(ctx context.Context, req *BatchGetArticlesRequest) (*BatchGetArticlesResponse, error) {
	// Ensure request ID exists
	ctx, requestID := EnsureRequestID(ctx)
	serviceStart := time.Now()

	s.logger.Info("[BatchGetArticles] started",
		slog.String("request_id", requestID),
		slog.String("appid", req.AuthorizerAppID),
		slog.Int("offset", req.Offset),
		slog.Int("count", req.Count),
	)

	// Get authorizer token
	tokenStart := time.Now()
	token, err := s.tokenService.GetAuthorizerToken(ctx, req.AuthorizerAppID)
	tokenDuration := time.Since(tokenStart)

	if err != nil {
		s.logger.Error("[BatchGetArticles] failed to get token",
			slog.String("request_id", requestID),
			slog.String("appid", req.AuthorizerAppID),
			slog.Duration("token_duration", tokenDuration),
			slog.Duration("total_duration", time.Since(serviceStart)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get authorizer token: %w", err)
	}

	s.logger.Debug("[BatchGetArticles] token acquired",
		slog.String("request_id", requestID),
		slog.Duration("token_duration", tokenDuration),
	)

	// Call WeChat API
	wechatReq := &wechat.BatchGetRequest{
		Offset:    req.Offset,
		Count:     req.Count,
		NoContent: req.NoContent,
	}

	apiStart := time.Now()
	resp, err := s.wechatClient.BatchGetPublishedArticles(ctx, token, wechatReq)
	apiDuration := time.Since(apiStart)

	// Handle token expiry with retry
	if err != nil && isTokenExpiredError(err) {
		s.logger.Warn("[BatchGetArticles] token expired, retrying",
			slog.String("request_id", requestID),
			slog.String("appid", req.AuthorizerAppID),
			slog.Duration("api_duration", apiDuration),
			slog.String("original_error", err.Error()),
		)

		// Refresh token
		refreshStart := time.Now()
		token, err = s.tokenService.InvalidateAndRefreshToken(ctx, req.AuthorizerAppID)
		refreshDuration := time.Since(refreshStart)

		if err != nil {
			s.logger.Error("[BatchGetArticles] token refresh failed",
				slog.String("request_id", requestID),
				slog.String("appid", req.AuthorizerAppID),
				slog.Duration("refresh_duration", refreshDuration),
				slog.Duration("total_duration", time.Since(serviceStart)),
				slog.String("error", err.Error()),
			)
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		s.logger.Info("[BatchGetArticles] token refreshed, retrying API call",
			slog.String("request_id", requestID),
			slog.Duration("refresh_duration", refreshDuration),
		)

		// Retry API call
		retryStart := time.Now()
		resp, err = s.wechatClient.BatchGetPublishedArticles(ctx, token, wechatReq)
		retryDuration := time.Since(retryStart)

		if err != nil {
			s.logger.Error("[BatchGetArticles] retry failed",
				slog.String("request_id", requestID),
				slog.String("appid", req.AuthorizerAppID),
				slog.Duration("retry_api_duration", retryDuration),
				slog.Duration("total_duration", time.Since(serviceStart)),
				slog.String("error", err.Error()),
			)
		} else {
			s.logger.Info("[BatchGetArticles] retry succeeded",
				slog.String("request_id", requestID),
				slog.Duration("retry_api_duration", retryDuration),
			)
			apiDuration = retryDuration // Update for final log
		}
	}

	if err != nil {
		s.logger.Error("[BatchGetArticles] failed",
			slog.String("request_id", requestID),
			slog.String("appid", req.AuthorizerAppID),
			slog.Duration("api_duration", apiDuration),
			slog.Duration("total_duration", time.Since(serviceStart)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get published articles: %w", err)
	}

	totalDuration := time.Since(serviceStart)
	s.logger.Info("[BatchGetArticles] completed",
		slog.String("request_id", requestID),
		slog.String("appid", req.AuthorizerAppID),
		slog.Int("total_count", resp.TotalCount),
		slog.Int("item_count", resp.ItemCount),
		slog.Duration("token_duration", tokenDuration),
		slog.Duration("api_duration", apiDuration),
		slog.Duration("total_duration", totalDuration),
	)

	return &BatchGetArticlesResponse{
		TotalCount: resp.TotalCount,
		ItemCount:  resp.ItemCount,
		Item:       resp.Item,
	}, nil
}

// GetPublishedArticle gets article details.
func (s *ArticleServiceImpl) GetPublishedArticle(ctx context.Context, req *GetArticleRequest) (*GetArticleResponse, error) {
	// Ensure request ID exists
	ctx, requestID := EnsureRequestID(ctx)
	serviceStart := time.Now()

	s.logger.Info("[GetArticle] started",
		slog.String("request_id", requestID),
		slog.String("appid", req.AuthorizerAppID),
		slog.String("article_id", req.ArticleID),
	)

	// Get authorizer token
	tokenStart := time.Now()
	token, err := s.tokenService.GetAuthorizerToken(ctx, req.AuthorizerAppID)
	tokenDuration := time.Since(tokenStart)

	if err != nil {
		s.logger.Error("[GetArticle] failed to get token",
			slog.String("request_id", requestID),
			slog.String("appid", req.AuthorizerAppID),
			slog.Duration("token_duration", tokenDuration),
			slog.Duration("total_duration", time.Since(serviceStart)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get authorizer token: %w", err)
	}

	s.logger.Debug("[GetArticle] token acquired",
		slog.String("request_id", requestID),
		slog.Duration("token_duration", tokenDuration),
	)

	// Call WeChat API
	apiStart := time.Now()
	resp, err := s.wechatClient.GetPublishedArticle(ctx, token, req.ArticleID)
	apiDuration := time.Since(apiStart)

	// Handle token expiry with retry
	if err != nil && isTokenExpiredError(err) {
		s.logger.Warn("[GetArticle] token expired, retrying",
			slog.String("request_id", requestID),
			slog.String("appid", req.AuthorizerAppID),
			slog.String("article_id", req.ArticleID),
			slog.Duration("api_duration", apiDuration),
			slog.String("original_error", err.Error()),
		)

		// Refresh token
		refreshStart := time.Now()
		token, err = s.tokenService.InvalidateAndRefreshToken(ctx, req.AuthorizerAppID)
		refreshDuration := time.Since(refreshStart)

		if err != nil {
			s.logger.Error("[GetArticle] token refresh failed",
				slog.String("request_id", requestID),
				slog.String("appid", req.AuthorizerAppID),
				slog.Duration("refresh_duration", refreshDuration),
				slog.Duration("total_duration", time.Since(serviceStart)),
				slog.String("error", err.Error()),
			)
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		s.logger.Info("[GetArticle] token refreshed, retrying API call",
			slog.String("request_id", requestID),
			slog.Duration("refresh_duration", refreshDuration),
		)

		// Retry API call
		retryStart := time.Now()
		resp, err = s.wechatClient.GetPublishedArticle(ctx, token, req.ArticleID)
		retryDuration := time.Since(retryStart)

		if err != nil {
			s.logger.Error("[GetArticle] retry failed",
				slog.String("request_id", requestID),
				slog.String("appid", req.AuthorizerAppID),
				slog.String("article_id", req.ArticleID),
				slog.Duration("retry_api_duration", retryDuration),
				slog.Duration("total_duration", time.Since(serviceStart)),
				slog.String("error", err.Error()),
			)
		} else {
			s.logger.Info("[GetArticle] retry succeeded",
				slog.String("request_id", requestID),
				slog.Duration("retry_api_duration", retryDuration),
			)
			apiDuration = retryDuration
		}
	}

	if err != nil {
		s.logger.Error("[GetArticle] failed",
			slog.String("request_id", requestID),
			slog.String("appid", req.AuthorizerAppID),
			slog.String("article_id", req.ArticleID),
			slog.Duration("api_duration", apiDuration),
			slog.Duration("total_duration", time.Since(serviceStart)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get article: %w", err)
	}

	totalDuration := time.Since(serviceStart)
	s.logger.Info("[GetArticle] completed",
		slog.String("request_id", requestID),
		slog.String("appid", req.AuthorizerAppID),
		slog.String("article_id", req.ArticleID),
		slog.Int("news_item_count", len(resp.NewsItem)),
		slog.Duration("token_duration", tokenDuration),
		slog.Duration("api_duration", apiDuration),
		slog.Duration("total_duration", totalDuration),
	)

	return &GetArticleResponse{
		NewsItem: resp.NewsItem,
	}, nil
}

// isTokenExpiredError checks if the error indicates token expiration.
func isTokenExpiredError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "code=40001") ||
		strings.Contains(errMsg, "code=42001")
}
