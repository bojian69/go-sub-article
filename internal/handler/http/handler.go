// Package http provides HTTP handler implementation.
package http

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/handler"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/repository/cache"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/service"
)

// Error codes following uhomes standard
const (
	CodeSuccess      = 0
	CodeInvalidParam = 400001
	CodeNotFound     = 404001
	CodeInternalErr  = 500001
)

// StandardResponse represents the standard API response structure.
type StandardResponse struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	RequestID string      `json:"request_id"`
	Data      interface{} `json:"data,omitempty"`
	Metadata  interface{} `json:"metadata,omitempty"`
}

// Handler implements the HTTP handlers.
type Handler struct {
	articleService               service.ArticleService
	cacheRepo                    cache.Repository
	wechatCallbackHandler        *handler.WeChatCallbackHandler
	subscriptionMessageHandler   *handler.SubscriptionMessageHandler
	validate                     *validator.Validate
	logger                       *slog.Logger
}

// NewHandler creates a new HTTP handler.
func NewHandler(
	articleService service.ArticleService,
	cacheRepo cache.Repository,
	wechatCallbackHandler *handler.WeChatCallbackHandler,
	subscriptionMessageHandler *handler.SubscriptionMessageHandler,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		articleService:             articleService,
		cacheRepo:                  cacheRepo,
		wechatCallbackHandler:      wechatCallbackHandler,
		subscriptionMessageHandler: subscriptionMessageHandler,
		validate:                   validator.New(),
		logger:                     logger,
	}
}

// RegisterRoutes registers all HTTP routes.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// Health check endpoint
	r.GET("/health", h.HealthCheck)

	// WeChat callback verification endpoint
	r.GET("/wechat/callback", h.WeChatCallback)

	// Serve static files for web UI
	r.StaticFile("/", "./web/index.html")
	r.StaticFile("/index.html", "./web/index.html")
	r.Static("/web", "./web")
	r.Static("/docs", "./docs")

	// API routes
	v1 := r.Group("/v1")
	{
		accounts := v1.Group("/accounts/:authorizer_appid")
		{
			accounts.GET("/articles", h.BatchGetArticles)
			accounts.GET("/articles/:article_id", h.GetArticle)
		}

		// Subscription message routes
		subscriptionMessage := v1.Group("/subscription-message")
		{
			subscriptionMessage.POST("/send", h.subscriptionMessageHandler.SendSubscriptionMessage)
			subscriptionMessage.GET("/templates", h.subscriptionMessageHandler.GetTemplateList)
		}
	}
}

// HealthCheck handles GET /health for container health probes.
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// BatchGetArticles handles GET /v1/accounts/:authorizer_appid/articles
func (h *Handler) BatchGetArticles(c *gin.Context) {
	requestID := uuid.New().String()
	c.Set("request_id", requestID)

	// Add requestID to context for service layer
	ctx := service.WithRequestID(c.Request.Context(), requestID)

	authorizerAppID := c.Param("authorizer_appid")

	h.logger.Info("[HTTP] BatchGetArticles request",
		slog.String("request_id", requestID),
		slog.String("authorizer_appid", authorizerAppID),
	)

	// Parse query parameters
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	count, _ := strconv.Atoi(c.DefaultQuery("count", "10"))
	noContent, _ := strconv.Atoi(c.DefaultQuery("no_content", "0"))

	// Validate parameters
	if authorizerAppID == "" {
		h.errorResponse(c, http.StatusBadRequest, CodeInvalidParam, "authorizer_appid is required", requestID)
		return
	}
	if offset < 0 {
		h.errorResponse(c, http.StatusBadRequest, CodeInvalidParam, "offset must be >= 0", requestID)
		return
	}
	if count < 1 || count > 20 {
		h.errorResponse(c, http.StatusBadRequest, CodeInvalidParam, "count must be between 1 and 20", requestID)
		return
	}
	if noContent != 0 && noContent != 1 {
		h.errorResponse(c, http.StatusBadRequest, CodeInvalidParam, "no_content must be 0 or 1", requestID)
		return
	}

	// Call service
	req := &service.BatchGetArticlesRequest{
		AuthorizerAppID: authorizerAppID,
		Offset:          offset,
		Count:           count,
		NoContent:       noContent,
	}

	resp, err := h.articleService.BatchGetPublishedArticles(ctx, req)
	if err != nil {
		h.logger.Error("[HTTP] service error",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		h.errorResponse(c, http.StatusInternalServerError, CodeInternalErr, "failed to get articles", requestID)
		return
	}

	h.logger.Info("[HTTP] BatchGetArticles success",
		slog.String("request_id", requestID),
		slog.Int("total_count", resp.TotalCount),
		slog.Int("item_count", resp.ItemCount),
	)

	h.successResponse(c, requestID, resp)
}

// GetArticle handles GET /v1/accounts/:authorizer_appid/articles/:article_id
func (h *Handler) GetArticle(c *gin.Context) {
	requestID := uuid.New().String()
	c.Set("request_id", requestID)

	// Add requestID to context for service layer
	ctx := service.WithRequestID(c.Request.Context(), requestID)

	authorizerAppID := c.Param("authorizer_appid")
	articleID := c.Param("article_id")

	h.logger.Info("[HTTP] GetArticle request",
		slog.String("request_id", requestID),
		slog.String("authorizer_appid", authorizerAppID),
		slog.String("article_id", articleID),
	)

	// Validate parameters
	if authorizerAppID == "" {
		h.errorResponse(c, http.StatusBadRequest, CodeInvalidParam, "authorizer_appid is required", requestID)
		return
	}
	if articleID == "" {
		h.errorResponse(c, http.StatusBadRequest, CodeInvalidParam, "article_id is required", requestID)
		return
	}

	// Call service
	req := &service.GetArticleRequest{
		AuthorizerAppID: authorizerAppID,
		ArticleID:       articleID,
	}

	resp, err := h.articleService.GetPublishedArticle(ctx, req)
	if err != nil {
		h.logger.Error("[HTTP] service error",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		h.errorResponse(c, http.StatusInternalServerError, CodeInternalErr, "failed to get article", requestID)
		return
	}

	h.logger.Info("[HTTP] GetArticle success",
		slog.String("request_id", requestID),
		slog.Int("news_item_count", len(resp.NewsItem)),
	)

	h.successResponse(c, requestID, resp)
}

// successResponse sends a successful response.
func (h *Handler) successResponse(c *gin.Context, requestID string, data interface{}) {
	c.JSON(http.StatusOK, StandardResponse{
		Code:      CodeSuccess,
		Message:   "success",
		RequestID: requestID,
		Data:      data,
	})
}

// errorResponse sends an error response.
func (h *Handler) errorResponse(c *gin.Context, httpStatus int, code int, message string, requestID string) {
	c.JSON(httpStatus, StandardResponse{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	})
}

// GenerateRequestID generates a unique request ID.
func GenerateRequestID() string {
	return uuid.New().String()
}

// WeChatCallback handles WeChat server verification requests.
func (h *Handler) WeChatCallback(c *gin.Context) {
	requestID := uuid.New().String()
	c.Set("request_id", requestID)

	h.logger.Info("[HTTP] WeChatCallback request",
		slog.String("request_id", requestID),
		slog.String("signature", c.Query("signature")),
		slog.String("timestamp", c.Query("timestamp")),
		slog.String("nonce", c.Query("nonce")),
	)

	// Parse query parameters
	var req handler.ServerVerificationRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Error("[HTTP] failed to parse query parameters",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		h.errorResponse(c, http.StatusBadRequest, CodeInvalidParam, "invalid parameters", requestID)
		return
	}

	// Call verification service
	resp, err := h.wechatCallbackHandler.VerifyServer(&req)
	if err != nil {
		h.logger.Error("[HTTP] verification service error",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		h.errorResponse(c, http.StatusInternalServerError, CodeInternalErr, "verification failed", requestID)
		return
	}

	// Check verification result
	if !resp.Valid {
		h.logger.Warn("[HTTP] verification failed",
			slog.String("request_id", requestID),
			slog.String("message", resp.Message),
		)
		c.String(http.StatusForbidden, "verification failed: %s", resp.Message)
		return
	}

	// Return echostr for successful verification
	h.logger.Info("[HTTP] verification success",
		slog.String("request_id", requestID),
	)
	c.String(http.StatusOK, resp.Echostr)
}
