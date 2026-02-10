// Package client provides WeChat API client implementation.
package client

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/sony/gobreaker/v2"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
)

// CircuitBreakerClient wraps a Client with circuit breaker protection.
type CircuitBreakerClient struct {
	inner  Client
	cb     *gobreaker.CircuitBreaker[any]
	logger *slog.Logger
}

// NewCircuitBreakerClient creates a new circuit breaker wrapped client.
func NewCircuitBreakerClient(inner Client, logger *slog.Logger) *CircuitBreakerClient {
	settings := gobreaker.Settings{
		Name:        "wechat-api",
		MaxRequests: 3,                // allow 3 requests in half-open state
		Interval:    0,                // never clear counts in closed state (reset on state change)
		Timeout:     60 * time.Second, // 60s in open state before half-open
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Open circuit after 5 consecutive failures
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Warn("[CircuitBreaker] state changed",
				slog.String("name", name),
				slog.String("from", from.String()),
				slog.String("to", to.String()),
			)
		},
	}

	return &CircuitBreakerClient{
		inner:  inner,
		cb:     gobreaker.NewCircuitBreaker[any](settings),
		logger: logger,
	}
}

// GetAccessToken obtains access_token with circuit breaker protection.
func (c *CircuitBreakerClient) GetAccessToken(ctx context.Context, appID, appSecret string) (*wechat.AccessTokenResponse, error) {
	result, err := c.cb.Execute(func() (any, error) {
		return c.inner.GetAccessToken(ctx, appID, appSecret)
	})
	if err != nil {
		return nil, c.wrapError(err)
	}
	return result.(*wechat.AccessTokenResponse), nil
}

// GetComponentAccessToken obtains component_access_token with circuit breaker protection.
func (c *CircuitBreakerClient) GetComponentAccessToken(ctx context.Context, req *wechat.ComponentTokenRequest) (*wechat.ComponentTokenResponse, error) {
	result, err := c.cb.Execute(func() (any, error) {
		return c.inner.GetComponentAccessToken(ctx, req)
	})
	if err != nil {
		return nil, c.wrapError(err)
	}
	return result.(*wechat.ComponentTokenResponse), nil
}

// RefreshAuthorizerToken refreshes authorizer_access_token with circuit breaker protection.
func (c *CircuitBreakerClient) RefreshAuthorizerToken(ctx context.Context, componentToken string, req *wechat.RefreshAuthorizerTokenRequest) (*wechat.RefreshAuthorizerTokenResponse, error) {
	result, err := c.cb.Execute(func() (any, error) {
		return c.inner.RefreshAuthorizerToken(ctx, componentToken, req)
	})
	if err != nil {
		return nil, c.wrapError(err)
	}
	return result.(*wechat.RefreshAuthorizerTokenResponse), nil
}

// BatchGetPublishedArticles gets published articles list with circuit breaker protection.
func (c *CircuitBreakerClient) BatchGetPublishedArticles(ctx context.Context, accessToken string, req *wechat.BatchGetRequest) (*wechat.BatchGetResponse, error) {
	result, err := c.cb.Execute(func() (any, error) {
		return c.inner.BatchGetPublishedArticles(ctx, accessToken, req)
	})
	if err != nil {
		return nil, c.wrapError(err)
	}
	return result.(*wechat.BatchGetResponse), nil
}

// GetPublishedArticle gets article details with circuit breaker protection.
func (c *CircuitBreakerClient) GetPublishedArticle(ctx context.Context, accessToken string, articleID string) (*wechat.GetArticleResponse, error) {
	result, err := c.cb.Execute(func() (any, error) {
		return c.inner.GetPublishedArticle(ctx, accessToken, articleID)
	})
	if err != nil {
		return nil, c.wrapError(err)
	}
	return result.(*wechat.GetArticleResponse), nil
}

// State returns the current circuit breaker state.
func (c *CircuitBreakerClient) State() gobreaker.State {
	return c.cb.State()
}

func (c *CircuitBreakerClient) wrapError(err error) error {
	if err == gobreaker.ErrOpenState {
		return fmt.Errorf("wechat api circuit breaker is open: %w", err)
	}
	if err == gobreaker.ErrTooManyRequests {
		return fmt.Errorf("wechat api circuit breaker: too many requests in half-open state: %w", err)
	}
	return err
}
