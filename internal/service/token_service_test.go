package service

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/config"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
)

// MockCacheRepository is a mock implementation of cache.Repository
type MockCacheRepository struct {
	componentTokens   map[string]string
	authorizerTokens  map[string]string
	ttls              map[string]time.Duration
	mu                sync.RWMutex
	getComponentCalls int32
	getAuthorizerCalls int32
}

func NewMockCacheRepository() *MockCacheRepository {
	return &MockCacheRepository{
		componentTokens:  make(map[string]string),
		authorizerTokens: make(map[string]string),
		ttls:             make(map[string]time.Duration),
	}
}

func (m *MockCacheRepository) GetComponentToken(ctx context.Context, componentAppID string) (string, error) {
	atomic.AddInt32(&m.getComponentCalls, 1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.componentTokens[componentAppID], nil
}

func (m *MockCacheRepository) SetComponentToken(ctx context.Context, componentAppID string, token string, expiresIn int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.componentTokens[componentAppID] = token
	return nil
}

func (m *MockCacheRepository) GetAuthorizerToken(ctx context.Context, authorizerAppID string) (string, error) {
	atomic.AddInt32(&m.getAuthorizerCalls, 1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.authorizerTokens[authorizerAppID], nil
}

func (m *MockCacheRepository) SetAuthorizerToken(ctx context.Context, authorizerAppID string, token string, expiresIn int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authorizerTokens[authorizerAppID] = token
	return nil
}

func (m *MockCacheRepository) GetTokenTTL(ctx context.Context, key string) (time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ttls[key], nil
}

func (m *MockCacheRepository) Close() error {
	return nil
}

func (m *MockCacheRepository) SetCachedToken(appID, token string, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authorizerTokens[appID] = token
	m.ttls["wechat:token:authorizer:"+appID] = ttl
}

func (m *MockCacheRepository) SetCachedComponentToken(appID, token string, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.componentTokens[appID] = token
	m.ttls["wechat:token:component:"+appID] = ttl
}

// MockWeChatClient is a mock implementation of client.Client
type MockWeChatClient struct {
	componentTokenResp   *wechat.ComponentTokenResponse
	authorizerTokenResp  *wechat.RefreshAuthorizerTokenResponse
	apiCallCount         int32
	mu                   sync.Mutex
}

func NewMockWeChatClient() *MockWeChatClient {
	return &MockWeChatClient{
		componentTokenResp: &wechat.ComponentTokenResponse{
			ComponentAccessToken: "mock_component_token",
			ExpiresIn:            7200,
		},
		authorizerTokenResp: &wechat.RefreshAuthorizerTokenResponse{
			AuthorizerAccessToken:  "mock_authorizer_token",
			ExpiresIn:              7200,
			AuthorizerRefreshToken: "mock_refresh_token",
		},
	}
}

func (m *MockWeChatClient) GetComponentAccessToken(ctx context.Context, req *wechat.ComponentTokenRequest) (*wechat.ComponentTokenResponse, error) {
	atomic.AddInt32(&m.apiCallCount, 1)
	return m.componentTokenResp, nil
}

func (m *MockWeChatClient) RefreshAuthorizerToken(ctx context.Context, componentToken string, req *wechat.RefreshAuthorizerTokenRequest) (*wechat.RefreshAuthorizerTokenResponse, error) {
	atomic.AddInt32(&m.apiCallCount, 1)
	return m.authorizerTokenResp, nil
}

func (m *MockWeChatClient) BatchGetPublishedArticles(ctx context.Context, accessToken string, req *wechat.BatchGetRequest) (*wechat.BatchGetResponse, error) {
	return &wechat.BatchGetResponse{}, nil
}

func (m *MockWeChatClient) GetPublishedArticle(ctx context.Context, accessToken string, articleID string) (*wechat.GetArticleResponse, error) {
	return &wechat.GetArticleResponse{}, nil
}

func (m *MockWeChatClient) GetAccessToken(ctx context.Context, appID, appSecret string) (*wechat.AccessTokenResponse, error) {
	atomic.AddInt32(&m.apiCallCount, 1)
	return &wechat.AccessTokenResponse{
		AccessToken: "mock_simple_access_token",
		ExpiresIn:   7200,
	}, nil
}

func (m *MockWeChatClient) GetAPICallCount() int32 {
	return atomic.LoadInt32(&m.apiCallCount)
}

func (m *MockWeChatClient) ResetAPICallCount() {
	atomic.StoreInt32(&m.apiCallCount, 0)
}

// Property 1: Token Cache-First Pattern
// For any token request, the Token_Manager SHALL first check the Redis cache.
// If a valid cached token exists, it SHALL be returned without API call.
// **Validates: Requirements 1.2, 1.3, 1.4, 1.5**
func TestProperty_TokenCacheFirstPattern(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: When token is cached, no API call is made
	properties.Property("cached token returns without API call", prop.ForAll(
		func(appID string) bool {
			if appID == "" {
				return true
			}

			cacheRepo := NewMockCacheRepository()
			wechatClient := NewMockWeChatClient()
			cfg := &config.WeChatConfig{
				Component: config.ComponentConfig{
					AppID:        "comp_appid",
					AppSecret:    "comp_secret",
					VerifyTicket: "comp_ticket",
				},
				Authorizers: []config.AuthorizerConfig{
					{AppID: appID, RefreshToken: "refresh_token"},
				},
			}

			// Pre-cache the token with long TTL
			cachedToken := "cached_token_" + appID
			cacheRepo.SetCachedToken(appID, cachedToken, 30*time.Minute)

			svc := NewTokenService(cfg, cacheRepo, wechatClient, slog.Default())
			ctx := context.Background()

			token, err := svc.GetAuthorizerToken(ctx, appID)
			if err != nil {
				return false
			}

			// Should return cached token
			if token != cachedToken {
				return false
			}

			// No API call should be made
			return wechatClient.GetAPICallCount() == 0
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
	))

	properties.TestingRun(t)
}

// Property 4: Singleflight Concurrency Control
// For any set of N concurrent requests for the same token, only one actual refresh API call SHALL be made.
// **Validates: Requirements 1.8**
func TestProperty_SingleflightConcurrencyControl(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: Concurrent requests result in single API call
	// Note: Due to goroutine scheduling, we use a barrier to ensure all goroutines start together
	properties.Property("concurrent requests make single API call", prop.ForAll(
		func(concurrency int) bool {
			if concurrency < 2 || concurrency > 20 {
				return true
			}

			cacheRepo := NewMockCacheRepository()
			wechatClient := NewMockWeChatClient()
			cfg := &config.WeChatConfig{
				Component: config.ComponentConfig{
					AppID:        "comp_appid",
					AppSecret:    "comp_secret",
					VerifyTicket: "comp_ticket",
				},
				Authorizers: []config.AuthorizerConfig{
					{AppID: "test_appid", RefreshToken: "refresh_token"},
				},
			}

			// Pre-cache component token to avoid extra API calls
			cacheRepo.SetCachedComponentToken("comp_appid", "comp_token", 30*time.Minute)

			svc := NewTokenService(cfg, cacheRepo, wechatClient, slog.Default())
			ctx := context.Background()

			var wg sync.WaitGroup
			results := make([]string, concurrency)
			errors := make([]error, concurrency)

			// Use a barrier to ensure all goroutines start at the same time
			startBarrier := make(chan struct{})

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					<-startBarrier // Wait for signal to start
					token, err := svc.GetAuthorizerToken(ctx, "test_appid")
					results[idx] = token
					errors[idx] = err
				}(i)
			}

			// Small delay to ensure all goroutines are waiting
			time.Sleep(10 * time.Millisecond)
			// Release all goroutines at once
			close(startBarrier)

			wg.Wait()

			// All requests should succeed
			for i := 0; i < concurrency; i++ {
				if errors[i] != nil {
					return false
				}
			}

			// All results should be the same
			for i := 1; i < concurrency; i++ {
				if results[i] != results[0] {
					return false
				}
			}

			// Only one API call should be made for authorizer token
			// (component token is cached, so only 1 call for authorizer)
			return wechatClient.GetAPICallCount() == 1
		},
		gen.IntRange(2, 10),
	))

	properties.TestingRun(t)
}

// Unit tests
func TestTokenService_GetAuthorizerToken_CacheHit(t *testing.T) {
	cacheRepo := NewMockCacheRepository()
	wechatClient := NewMockWeChatClient()
	cfg := &config.WeChatConfig{
		Component: config.ComponentConfig{
			AppID:        "comp_appid",
			AppSecret:    "comp_secret",
			VerifyTicket: "comp_ticket",
		},
		Authorizers: []config.AuthorizerConfig{
			{AppID: "auth_appid", RefreshToken: "refresh_token"},
		},
	}

	// Pre-cache the token
	cacheRepo.SetCachedToken("auth_appid", "cached_token", 30*time.Minute)

	svc := NewTokenService(cfg, cacheRepo, wechatClient, slog.Default())
	ctx := context.Background()

	token, err := svc.GetAuthorizerToken(ctx, "auth_appid")

	require.NoError(t, err)
	assert.Equal(t, "cached_token", token)
	assert.Equal(t, int32(0), wechatClient.GetAPICallCount())
}

func TestTokenService_GetAuthorizerToken_CacheMiss(t *testing.T) {
	cacheRepo := NewMockCacheRepository()
	wechatClient := NewMockWeChatClient()
	cfg := &config.WeChatConfig{
		Component: config.ComponentConfig{
			AppID:        "comp_appid",
			AppSecret:    "comp_secret",
			VerifyTicket: "comp_ticket",
		},
		Authorizers: []config.AuthorizerConfig{
			{AppID: "auth_appid", RefreshToken: "refresh_token"},
		},
	}

	// Pre-cache component token
	cacheRepo.SetCachedComponentToken("comp_appid", "comp_token", 30*time.Minute)

	svc := NewTokenService(cfg, cacheRepo, wechatClient, slog.Default())
	ctx := context.Background()

	token, err := svc.GetAuthorizerToken(ctx, "auth_appid")

	require.NoError(t, err)
	assert.Equal(t, "mock_authorizer_token", token)
	assert.Equal(t, int32(1), wechatClient.GetAPICallCount())
}

func TestTokenService_GetAuthorizerToken_NotFound(t *testing.T) {
	cacheRepo := NewMockCacheRepository()
	wechatClient := NewMockWeChatClient()
	cfg := &config.WeChatConfig{
		Component: config.ComponentConfig{
			AppID:        "comp_appid",
			AppSecret:    "comp_secret",
			VerifyTicket: "comp_ticket",
		},
		Authorizers: []config.AuthorizerConfig{
			{AppID: "auth_appid", RefreshToken: "refresh_token"},
		},
	}

	svc := NewTokenService(cfg, cacheRepo, wechatClient, slog.Default())
	ctx := context.Background()

	_, err := svc.GetAuthorizerToken(ctx, "unknown_appid")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authorizer not found")
}

func TestTokenService_GetComponentToken_CacheHit(t *testing.T) {
	cacheRepo := NewMockCacheRepository()
	wechatClient := NewMockWeChatClient()
	cfg := &config.WeChatConfig{
		Component: config.ComponentConfig{
			AppID:        "comp_appid",
			AppSecret:    "comp_secret",
			VerifyTicket: "comp_ticket",
		},
	}

	// Pre-cache the token
	cacheRepo.SetCachedComponentToken("comp_appid", "cached_comp_token", 30*time.Minute)

	svc := NewTokenService(cfg, cacheRepo, wechatClient, slog.Default())
	ctx := context.Background()

	token, err := svc.GetComponentToken(ctx)

	require.NoError(t, err)
	assert.Equal(t, "cached_comp_token", token)
	assert.Equal(t, int32(0), wechatClient.GetAPICallCount())
}

func TestTokenService_ConcurrentRequests(t *testing.T) {
	cacheRepo := NewMockCacheRepository()
	wechatClient := NewMockWeChatClient()
	cfg := &config.WeChatConfig{
		Component: config.ComponentConfig{
			AppID:        "comp_appid",
			AppSecret:    "comp_secret",
			VerifyTicket: "comp_ticket",
		},
		Authorizers: []config.AuthorizerConfig{
			{AppID: "auth_appid", RefreshToken: "refresh_token"},
		},
	}

	// Pre-cache component token
	cacheRepo.SetCachedComponentToken("comp_appid", "comp_token", 30*time.Minute)

	svc := NewTokenService(cfg, cacheRepo, wechatClient, slog.Default())
	ctx := context.Background()

	const concurrency = 10
	var wg sync.WaitGroup
	results := make([]string, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			token, _ := svc.GetAuthorizerToken(ctx, "auth_appid")
			results[idx] = token
		}(i)
	}

	wg.Wait()

	// All results should be the same
	for i := 1; i < concurrency; i++ {
		assert.Equal(t, results[0], results[i])
	}

	// Only one API call should be made
	assert.Equal(t, int32(1), wechatClient.GetAPICallCount())
}
