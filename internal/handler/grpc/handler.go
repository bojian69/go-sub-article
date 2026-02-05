// Package grpc provides gRPC handler implementation.
package grpc

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "git.uhomes.net/uhs-go/wechat-subscription-svc/api/proto"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/service"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
)

// Handler implements the gRPC SubscriptionService.
type Handler struct {
	pb.UnimplementedSubscriptionServiceServer
	articleService service.ArticleService
	logger         *slog.Logger
}

// NewHandler creates a new gRPC handler.
func NewHandler(articleService service.ArticleService, logger *slog.Logger) *Handler {
	return &Handler{
		articleService: articleService,
		logger:         logger,
	}
}

// BatchGetPublishedArticles implements the BatchGetPublishedArticles RPC.
func (h *Handler) BatchGetPublishedArticles(ctx context.Context, req *pb.BatchGetArticlesRequest) (*pb.BatchGetArticlesResponse, error) {
	requestID := uuid.New().String()

	// Set request_id in response metadata
	if err := grpc.SetHeader(ctx, metadata.Pairs("x-request-id", requestID)); err != nil {
		h.logger.Warn("failed to set response header", slog.String("error", err.Error()))
	}

	h.logger.Info("BatchGetPublishedArticles request",
		slog.String("request_id", requestID),
		slog.String("authorizer_appid", req.GetAuthorizerAppid()),
		slog.Int("offset", int(req.GetOffset())),
		slog.Int("count", int(req.GetCount())),
	)

	// Validate request
	if err := h.validateBatchGetRequest(req); err != nil {
		h.logger.Warn("validation failed",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	// Call service
	svcReq := &service.BatchGetArticlesRequest{
		AuthorizerAppID: req.GetAuthorizerAppid(),
		Offset:          int(req.GetOffset()),
		Count:           int(req.GetCount()),
		NoContent:       int(req.GetNoContent()),
	}

	resp, err := h.articleService.BatchGetPublishedArticles(ctx, svcReq)
	if err != nil {
		h.logger.Error("service error",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		return nil, status.Errorf(codes.Internal, "failed to get articles: %v", err)
	}

	// Convert response
	pbResp := &pb.BatchGetArticlesResponse{
		TotalCount: int32(resp.TotalCount),
		ItemCount:  int32(resp.ItemCount),
		Item:       convertPublishedArticles(resp.Item),
	}

	h.logger.Info("BatchGetPublishedArticles success",
		slog.String("request_id", requestID),
		slog.Int("total_count", resp.TotalCount),
		slog.Int("item_count", resp.ItemCount),
	)

	return pbResp, nil
}

// GetPublishedArticle implements the GetPublishedArticle RPC.
func (h *Handler) GetPublishedArticle(ctx context.Context, req *pb.GetArticleRequest) (*pb.GetArticleResponse, error) {
	requestID := uuid.New().String()

	// Set request_id in response metadata
	if err := grpc.SetHeader(ctx, metadata.Pairs("x-request-id", requestID)); err != nil {
		h.logger.Warn("failed to set response header", slog.String("error", err.Error()))
	}

	h.logger.Info("GetPublishedArticle request",
		slog.String("request_id", requestID),
		slog.String("authorizer_appid", req.GetAuthorizerAppid()),
		slog.String("article_id", req.GetArticleId()),
	)

	// Validate request
	if err := h.validateGetArticleRequest(req); err != nil {
		h.logger.Warn("validation failed",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	// Call service
	svcReq := &service.GetArticleRequest{
		AuthorizerAppID: req.GetAuthorizerAppid(),
		ArticleID:       req.GetArticleId(),
	}

	resp, err := h.articleService.GetPublishedArticle(ctx, svcReq)
	if err != nil {
		h.logger.Error("service error",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		return nil, status.Errorf(codes.Internal, "failed to get article: %v", err)
	}

	// Convert response
	pbResp := &pb.GetArticleResponse{
		NewsItem: convertNewsItems(resp.NewsItem),
	}

	h.logger.Info("GetPublishedArticle success",
		slog.String("request_id", requestID),
		slog.Int("news_item_count", len(resp.NewsItem)),
	)

	return pbResp, nil
}

// validateBatchGetRequest validates the BatchGetArticlesRequest.
func (h *Handler) validateBatchGetRequest(req *pb.BatchGetArticlesRequest) error {
	if req.GetAuthorizerAppid() == "" {
		return status.Error(codes.InvalidArgument, "authorizer_appid is required")
	}
	if req.GetOffset() < 0 {
		return status.Error(codes.InvalidArgument, "offset must be >= 0")
	}
	if req.GetCount() < 1 || req.GetCount() > 20 {
		return status.Error(codes.InvalidArgument, "count must be between 1 and 20")
	}
	if req.GetNoContent() != 0 && req.GetNoContent() != 1 {
		return status.Error(codes.InvalidArgument, "no_content must be 0 or 1")
	}
	return nil
}

// validateGetArticleRequest validates the GetArticleRequest.
func (h *Handler) validateGetArticleRequest(req *pb.GetArticleRequest) error {
	if req.GetAuthorizerAppid() == "" {
		return status.Error(codes.InvalidArgument, "authorizer_appid is required")
	}
	if req.GetArticleId() == "" {
		return status.Error(codes.InvalidArgument, "article_id is required")
	}
	return nil
}

// convertPublishedArticles converts service articles to protobuf articles.
func convertPublishedArticles(articles []wechat.PublishedArticle) []*pb.PublishedArticle {
	result := make([]*pb.PublishedArticle, len(articles))
	for i, article := range articles {
		result[i] = &pb.PublishedArticle{
			ArticleId:  article.ArticleID,
			UpdateTime: article.UpdateTime,
		}
		if article.Content != nil {
			result[i].Content = &pb.ArticleContent{
				NewsItem: convertNewsItems(article.Content.NewsItem),
			}
		}
	}
	return result
}

// convertNewsItems converts service news items to protobuf news items.
func convertNewsItems(items []wechat.NewsItem) []*pb.NewsItem {
	result := make([]*pb.NewsItem, len(items))
	for i, item := range items {
		result[i] = &pb.NewsItem{
			Title:              item.Title,
			Author:             item.Author,
			Digest:             item.Digest,
			Content:            item.Content,
			ContentSourceUrl:   item.ContentSourceURL,
			ThumbMediaId:       item.ThumbMediaID,
			ThumbUrl:           item.ThumbURL,
			NeedOpenComment:    int32(item.NeedOpenComment),
			OnlyFansCanComment: int32(item.OnlyFansCanComment),
			Url:                item.URL,
			IsDeleted:          item.IsDeleted,
		}
	}
	return result
}
