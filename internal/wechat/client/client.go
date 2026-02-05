// Package client provides WeChat API client implementation.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
)

const (
	// DefaultBaseURL is the WeChat API base URL
	DefaultBaseURL = "https://api.weixin.qq.com"

	// DefaultMaxRetries is the default maximum number of retries
	DefaultMaxRetries = 3

	// DefaultTimeout is the default HTTP client timeout
	DefaultTimeout = 10 * time.Second

	// InitialBackoff is the initial backoff duration for retries
	InitialBackoff = 100 * time.Millisecond

	// MaxBackoff is the maximum backoff duration
	MaxBackoff = 5 * time.Second

	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier = 2.0
)

// Client defines the WeChat API client interface.
type Client interface {
	// GetAccessToken obtains access_token directly using appid/appsecret (simple mode)
	GetAccessToken(ctx context.Context, appID, appSecret string) (*wechat.AccessTokenResponse, error)

	// GetComponentAccessToken obtains component_access_token
	GetComponentAccessToken(ctx context.Context, req *wechat.ComponentTokenRequest) (*wechat.ComponentTokenResponse, error)

	// RefreshAuthorizerToken refreshes authorizer_access_token
	RefreshAuthorizerToken(ctx context.Context, componentToken string, req *wechat.RefreshAuthorizerTokenRequest) (*wechat.RefreshAuthorizerTokenResponse, error)

	// BatchGetPublishedArticles gets published articles list
	BatchGetPublishedArticles(ctx context.Context, accessToken string, req *wechat.BatchGetRequest) (*wechat.BatchGetResponse, error)

	// GetPublishedArticle gets article details
	GetPublishedArticle(ctx context.Context, accessToken string, articleID string) (*wechat.GetArticleResponse, error)
}

// HTTPClient implements Client using HTTP.
type HTTPClient struct {
	httpClient *http.Client
	baseURL    string
	maxRetries int
	logger     *slog.Logger
}

// Option is a function that configures HTTPClient.
type Option func(*HTTPClient)

// WithBaseURL sets the base URL.
func WithBaseURL(url string) Option {
	return func(c *HTTPClient) {
		c.baseURL = url
	}
}

// WithMaxRetries sets the maximum number of retries.
func WithMaxRetries(retries int) Option {
	return func(c *HTTPClient) {
		c.maxRetries = retries
	}
}

// WithHTTPClient sets the HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *HTTPClient) {
		c.httpClient = client
	}
}

// WithLogger sets the logger.
func WithLogger(logger *slog.Logger) Option {
	return func(c *HTTPClient) {
		c.logger = logger
	}
}

// NewHTTPClient creates a new WeChat HTTP client.
func NewHTTPClient(opts ...Option) *HTTPClient {
	c := &HTTPClient{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		baseURL:    DefaultBaseURL,
		maxRetries: DefaultMaxRetries,
		logger:     slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// GetAccessToken obtains access_token directly using appid/appsecret (simple mode).
func (c *HTTPClient) GetAccessToken(ctx context.Context, appID, appSecret string) (*wechat.AccessTokenResponse, error) {
	url := fmt.Sprintf("%s/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s", c.baseURL, appID, appSecret)

	var resp wechat.AccessTokenResponse
	if err := c.doRequestWithRetry(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}

	// Check for WeChat API error
	if resp.ErrCode != 0 {
		c.logger.Error("WeChat API error",
			slog.Int("errcode", resp.ErrCode),
			slog.String("errmsg", resp.ErrMsg),
		)
		return nil, fmt.Errorf("wechat api error: code=%d, msg=%s", resp.ErrCode, resp.ErrMsg)
	}

	return &resp, nil
}

// GetComponentAccessToken obtains component_access_token.
func (c *HTTPClient) GetComponentAccessToken(ctx context.Context, req *wechat.ComponentTokenRequest) (*wechat.ComponentTokenResponse, error) {
	url := fmt.Sprintf("%s/cgi-bin/component/api_component_token", c.baseURL)

	var resp wechat.ComponentTokenResponse
	if err := c.doRequestWithRetry(ctx, http.MethodPost, url, req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// RefreshAuthorizerToken refreshes authorizer_access_token.
func (c *HTTPClient) RefreshAuthorizerToken(ctx context.Context, componentToken string, req *wechat.RefreshAuthorizerTokenRequest) (*wechat.RefreshAuthorizerTokenResponse, error) {
	url := fmt.Sprintf("%s/cgi-bin/component/api_authorizer_token?component_access_token=%s", c.baseURL, componentToken)

	var resp wechat.RefreshAuthorizerTokenResponse
	if err := c.doRequestWithRetry(ctx, http.MethodPost, url, req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// BatchGetPublishedArticles gets published articles list.
func (c *HTTPClient) BatchGetPublishedArticles(ctx context.Context, accessToken string, req *wechat.BatchGetRequest) (*wechat.BatchGetResponse, error) {
	url := fmt.Sprintf("%s/cgi-bin/freepublish/batchget?access_token=%s", c.baseURL, accessToken)

	var resp wechat.BatchGetResponse
	if err := c.doRequestWithRetry(ctx, http.MethodPost, url, req, &resp); err != nil {
		return nil, err
	}

	// Check for WeChat API error
	if resp.ErrCode != 0 {
		c.logger.Error("WeChat API error",
			slog.Int("errcode", resp.ErrCode),
			slog.String("errmsg", resp.ErrMsg),
		)
		return nil, fmt.Errorf("wechat api error: code=%d, msg=%s", resp.ErrCode, resp.ErrMsg)
	}

	return &resp, nil
}

// GetPublishedArticle gets article details.
func (c *HTTPClient) GetPublishedArticle(ctx context.Context, accessToken string, articleID string) (*wechat.GetArticleResponse, error) {
	url := fmt.Sprintf("%s/cgi-bin/freepublish/getarticle?access_token=%s", c.baseURL, accessToken)

	req := &wechat.GetArticleRequest{ArticleID: articleID}

	var resp wechat.GetArticleResponse
	if err := c.doRequestWithRetry(ctx, http.MethodPost, url, req, &resp); err != nil {
		return nil, err
	}

	// Check for WeChat API error
	if resp.ErrCode != 0 {
		c.logger.Error("WeChat API error",
			slog.Int("errcode", resp.ErrCode),
			slog.String("errmsg", resp.ErrMsg),
		)
		return nil, fmt.Errorf("wechat api error: code=%d, msg=%s", resp.ErrCode, resp.ErrMsg)
	}

	return &resp, nil
}

// doRequestWithRetry performs HTTP request with retry logic.
func (c *HTTPClient) doRequestWithRetry(ctx context.Context, method, url string, body interface{}, result interface{}) error {
	var lastErr error
	backoff := InitialBackoff

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Debug("retrying request",
				slog.Int("attempt", attempt),
				slog.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			// Exponential backoff
			backoff = time.Duration(float64(backoff) * BackoffMultiplier)
			if backoff > MaxBackoff {
				backoff = MaxBackoff
			}
		}

		err := c.doRequest(ctx, method, url, body, result)
		if err == nil {
			return nil
		}

		lastErr = err
		c.logger.Warn("request failed",
			slog.Int("attempt", attempt+1),
			slog.String("error", err.Error()),
		)
	}

	return fmt.Errorf("all retries exhausted: %w", lastErr)
}

// doRequest performs a single HTTP request.
func (c *HTTPClient) doRequest(ctx context.Context, method, url string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)

		c.logger.Debug("sending request",
			slog.String("method", method),
			slog.String("url", url),
		)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Debug("received response",
		slog.Int("status", resp.StatusCode),
	)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// GetRetryCount returns the number of retries that were made.
// This is useful for testing.
func (c *HTTPClient) GetRetryCount() int {
	return c.maxRetries
}
