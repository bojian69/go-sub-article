package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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
	TotalCount int                      `json:"total_count"`
	ItemCount  int                      `json:"item_count"`
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
	// Get authorizer token
	token, err := s.tokenService.GetAuthorizerToken(ctx, req.AuthorizerAppID)
	if err != nil {
		s.logger.Error("failed to get authorizer token",
			slog.String("authorizer_appid", req.AuthorizerAppID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get authorizer token: %w", err)
	}

	// Call WeChat API
	wechatReq := &wechat.BatchGetRequest{
		Offset:    req.Offset,
		Count:     req.Count,
		NoContent: req.NoContent,
	}

	resp, err := s.wechatClient.BatchGetPublishedArticles(ctx, token, wechatReq)
	if err != nil {
		// Check if token expired, retry with fresh token
		if isTokenExpiredError(err) {
			s.logger.Warn("token expired, refreshing and retrying",
				slog.String("authorizer_appid", req.AuthorizerAppID),
			)
			token, err = s.tokenService.InvalidateAndRefreshToken(ctx, req.AuthorizerAppID)
			if err != nil {
				return nil, fmt.Errorf("failed to refresh token: %w", err)
			}
			resp, err = s.wechatClient.BatchGetPublishedArticles(ctx, token, wechatReq)
		}
		if err != nil {
			s.logger.Error("failed to get published articles",
				slog.String("authorizer_appid", req.AuthorizerAppID),
				slog.String("error", err.Error()),
			)
			return nil, fmt.Errorf("failed to get published articles: %w", err)
		}
	}

	s.logger.Debug("got published articles",
		slog.String("authorizer_appid", req.AuthorizerAppID),
		slog.Int("total_count", resp.TotalCount),
		slog.Int("item_count", resp.ItemCount),
	)

	// Return response transparently
	return &BatchGetArticlesResponse{
		TotalCount: resp.TotalCount,
		ItemCount:  resp.ItemCount,
		Item:       resp.Item,
	}, nil
}

// GetPublishedArticle gets article details.
func (s *ArticleServiceImpl) GetPublishedArticle(ctx context.Context, req *GetArticleRequest) (*GetArticleResponse, error) {
	// Get authorizer token
	token, err := s.tokenService.GetAuthorizerToken(ctx, req.AuthorizerAppID)
	if err != nil {
		s.logger.Error("failed to get authorizer token",
			slog.String("authorizer_appid", req.AuthorizerAppID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get authorizer token: %w", err)
	}

	// Call WeChat API
	resp, err := s.wechatClient.GetPublishedArticle(ctx, token, req.ArticleID)
	if err != nil {
		// Check if token expired, retry with fresh token
		if isTokenExpiredError(err) {
			s.logger.Warn("token expired, refreshing and retrying",
				slog.String("authorizer_appid", req.AuthorizerAppID),
			)
			token, err = s.tokenService.InvalidateAndRefreshToken(ctx, req.AuthorizerAppID)
			if err != nil {
				return nil, fmt.Errorf("failed to refresh token: %w", err)
			}
			resp, err = s.wechatClient.GetPublishedArticle(ctx, token, req.ArticleID)
		}
		if err != nil {
			s.logger.Error("failed to get article",
				slog.String("authorizer_appid", req.AuthorizerAppID),
				slog.String("article_id", req.ArticleID),
				slog.String("error", err.Error()),
			)
			return nil, fmt.Errorf("failed to get article: %w", err)
		}
	}

	s.logger.Debug("got article details",
		slog.String("authorizer_appid", req.AuthorizerAppID),
		slog.String("article_id", req.ArticleID),
		slog.Int("news_item_count", len(resp.NewsItem)),
	)

	// Return response transparently
	return &GetArticleResponse{
		NewsItem: resp.NewsItem,
	}, nil
}

// isTokenExpiredError checks if the error indicates token expiration.
// WeChat API returns error codes 40001 (invalid credential) or 42001 (access_token expired)
func isTokenExpiredError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// Check for WeChat token expired error codes
	return strings.Contains(errMsg, "code=40001") ||
		strings.Contains(errMsg, "code=42001")
}
