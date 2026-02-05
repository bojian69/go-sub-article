// Package service provides business logic services.
package service

import (
	"context"

	"github.com/google/uuid"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/logger"
)

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return logger.WithRequestID(ctx, requestID)
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	return logger.GetRequestID(ctx)
}

// EnsureRequestID ensures a request ID exists in context, generating one if needed.
func EnsureRequestID(ctx context.Context) (context.Context, string) {
	if id := GetRequestID(ctx); id != "" {
		return ctx, id
	}
	id := uuid.New().String()
	return WithRequestID(ctx, id), id
}

// WithTraceID adds trace ID to context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return logger.WithTraceID(ctx, traceID)
}

// GetTraceID retrieves trace ID from context.
func GetTraceID(ctx context.Context) string {
	return logger.GetTraceID(ctx)
}

// WithSpanID adds span ID to context.
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return logger.WithSpanID(ctx, spanID)
}

// GetSpanID retrieves span ID from context.
func GetSpanID(ctx context.Context) string {
	return logger.GetSpanID(ctx)
}
