// Package handler provides HTTP handlers for the WeChat subscription service.
package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/service"
)

// SubscriptionMessageHandler handles subscription message HTTP requests.
type SubscriptionMessageHandler struct {
	service service.SubscriptionMessageService
	logger  *slog.Logger
}

// NewSubscriptionMessageHandler creates a new subscription message handler.
func NewSubscriptionMessageHandler(
	service service.SubscriptionMessageService,
	logger *slog.Logger,
) *SubscriptionMessageHandler {
	return &SubscriptionMessageHandler{
		service: service,
		logger:  logger,
	}
}

// SendSubscriptionMessage handles POST /api/v1/subscription-message/send
func (h *SubscriptionMessageHandler) SendSubscriptionMessage(c *gin.Context) {
	requestID := uuid.New().String()
	c.Set("request_id", requestID)

	// Add requestID to context for service layer
	ctx := service.WithRequestID(c.Request.Context(), requestID)

	h.logger.Info("[HTTP] SendSubscriptionMessage request",
		slog.String("request_id", requestID),
	)

	// Parse JSON request body
	var req service.SendSubscriptionMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("[HTTP] failed to parse request body",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":       400001,
			"message":    "invalid request body",
			"request_id": requestID,
		})
		return
	}

	h.logger.Info("[HTTP] SendSubscriptionMessage parsed request",
		slog.String("request_id", requestID),
		slog.String("authorizer_appid", req.AuthorizerAppID),
		slog.String("openid", req.OpenID),
		slog.String("template_id", req.TemplateID),
	)

	// Call service layer
	resp, err := h.service.SendSubscriptionMessage(ctx, &req)
	if err != nil {
		h.logger.Error("[HTTP] service error",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":       500001,
			"message":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	h.logger.Info("[HTTP] SendSubscriptionMessage success",
		slog.String("request_id", requestID),
		slog.Int64("msgid", resp.MsgID),
	)

	// Return JSON response
	c.JSON(http.StatusOK, gin.H{
		"code":       0,
		"message":    "success",
		"request_id": requestID,
		"data":       resp,
	})
}

// GetTemplateList handles GET /api/v1/subscription-message/templates
func (h *SubscriptionMessageHandler) GetTemplateList(c *gin.Context) {
	requestID := uuid.New().String()
	c.Set("request_id", requestID)

	// Add requestID to context for service layer
	ctx := service.WithRequestID(c.Request.Context(), requestID)

	h.logger.Info("[HTTP] GetTemplateList request",
		slog.String("request_id", requestID),
	)

	// Parse query parameter
	authorizerAppID := c.Query("authorizer_appid")
	if authorizerAppID == "" {
		h.logger.Error("[HTTP] missing authorizer_appid parameter",
			slog.String("request_id", requestID),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":       400002,
			"message":    "missing required parameter: authorizer_appid",
			"request_id": requestID,
		})
		return
	}

	h.logger.Info("[HTTP] GetTemplateList parsed request",
		slog.String("request_id", requestID),
		slog.String("authorizer_appid", authorizerAppID),
	)

	// Create service request
	req := &service.GetTemplateListRequest{
		AuthorizerAppID: authorizerAppID,
	}

	// Call service layer
	resp, err := h.service.GetTemplateList(ctx, req)
	if err != nil {
		h.logger.Error("[HTTP] service error",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":       500002,
			"message":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	h.logger.Info("[HTTP] GetTemplateList success",
		slog.String("request_id", requestID),
		slog.Int("template_count", len(resp.Templates)),
	)

	// Return JSON response
	c.JSON(http.StatusOK, gin.H{
		"code":       0,
		"message":    "success",
		"request_id": requestID,
		"data":       resp,
	})
}
