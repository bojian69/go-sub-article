// Package handler provides HTTP and gRPC handlers for the WeChat subscription service.
package handler

import (
	"crypto/sha1"
	"encoding/hex"
	"log/slog"
	"sort"
	"strings"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/config"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/service"
)

// ServerVerificationRequest represents the WeChat server verification request.
type ServerVerificationRequest struct {
	Signature       string `form:"signature" binding:"required"`
	Timestamp       string `form:"timestamp" binding:"required"`
	Nonce           string `form:"nonce" binding:"required"`
	Echostr         string `form:"echostr" binding:"required"`
	AuthorizerAppID string `form:"authorizer_appid"` // Optional: for multi-account support
}

// ServerVerificationResponse represents the WeChat server verification response.
type ServerVerificationResponse struct {
	Echostr string `json:"echostr,omitempty"`
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
}

// WeChatCallbackHandler handles WeChat callback verification and message processing.
type WeChatCallbackHandler struct {
	config *config.Config
	logger *slog.Logger
}

// NewWeChatCallbackHandler creates a new WeChat callback handler.
func NewWeChatCallbackHandler(cfg *config.Config, logger *slog.Logger) *WeChatCallbackHandler {
	return &WeChatCallbackHandler{
		config: cfg,
		logger: logger,
	}
}

// VerifyServer verifies the WeChat server request signature.
func (h *WeChatCallbackHandler) VerifyServer(req *ServerVerificationRequest) (*ServerVerificationResponse, error) {
	requestID := service.GenerateRequestID()

	h.logger.Info("[VerifyServer] started",
		slog.String("request_id", requestID),
		slog.String("timestamp", req.Timestamp),
		slog.String("nonce", req.Nonce),
		slog.String("authorizer_appid", req.AuthorizerAppID),
	)

	// Get configured callback token
	token := h.getCallbackToken(req.AuthorizerAppID)
	if token == "" {
		h.logger.Error("[VerifyServer] callback token not configured",
			slog.String("request_id", requestID),
		)
		return &ServerVerificationResponse{
			Valid:   false,
			Message: "callback token not configured",
		}, nil
	}

	// Calculate signature
	// Sort token, timestamp, nonce in lexicographical order
	params := []string{token, req.Timestamp, req.Nonce}
	sort.Strings(params)

	// Join and calculate SHA1 hash
	str := strings.Join(params, "")
	hash := sha1.New()
	hash.Write([]byte(str))
	calculatedSignature := hex.EncodeToString(hash.Sum(nil))

	// Verify signature
	if calculatedSignature != req.Signature {
		h.logger.Warn("[VerifyServer] signature mismatch",
			slog.String("request_id", requestID),
			slog.String("expected", calculatedSignature),
			slog.String("received", req.Signature),
		)
		return &ServerVerificationResponse{
			Valid:   false,
			Message: "signature verification failed",
		}, nil
	}

	// Signature verification successful
	h.logger.Info("[VerifyServer] verification success",
		slog.String("request_id", requestID),
	)

	return &ServerVerificationResponse{
		Echostr: req.Echostr,
		Valid:   true,
	}, nil
}

// getCallbackToken retrieves the callback token from configuration.
// In the future, this can be extended to support different tokens for different accounts.
func (h *WeChatCallbackHandler) getCallbackToken(appID string) string {
	// For now, return the unified callback token
	// TODO: Support per-account tokens if needed
	return h.config.WeChat.CallbackToken
}
