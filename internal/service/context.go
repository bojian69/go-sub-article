// Package service provides business logic services.
package service

import (
	"context"

	"github.com/google/uuid"
)

// Context keys
type contextKey string

const (
	requestIDKey contextKey = "request_id"
)

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// EnsureRequestID ensures a request ID exists in context, generating one if needed.
func EnsureRequestID(ctx context.Context) (context.Context, string) {
	if id := GetRequestID(ctx); id != "" {
		return ctx, id
	}
	id := uuid.New().String()
	return WithRequestID(ctx, id), id
}
