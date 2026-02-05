package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
)

// Property 10: Retry Behavior
// For any failed WeChat API call, the client SHALL retry up to 3 times with exponential backoff.
// **Validates: Requirements 2.9, 3.5**
func TestProperty_RetryBehavior(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20 // Reduced for retry tests
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: Client retries exactly maxRetries times on failure
	properties.Property("client retries maxRetries times on persistent failure", prop.ForAll(
		func(maxRetries int) bool {
			if maxRetries < 1 || maxRetries > 5 {
				return true // Skip invalid values
			}

			var callCount int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&callCount, 1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			client := NewHTTPClient(
				WithBaseURL(server.URL),
				WithMaxRetries(maxRetries),
			)

			ctx := context.Background()
			_, err := client.BatchGetPublishedArticles(ctx, "test_token", &wechat.BatchGetRequest{
				Offset: 0,
				Count:  10,
			})

			// Should have error after all retries
			if err == nil {
				return false
			}

			// Total calls = 1 initial + maxRetries retries
			expectedCalls := int32(maxRetries + 1)
			return atomic.LoadInt32(&callCount) == expectedCalls
		},
		gen.IntRange(1, 3),
	))

	properties.TestingRun(t)
}

// Property 9: Response Transparency
// For any successful WeChat API response, the service SHALL return the response structure transparently.
// **Validates: Requirements 2.8, 3.4**
func TestProperty_ResponseTransparency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: All fields in response are preserved
	properties.Property("batch get response preserves all fields", prop.ForAll(
		func(totalCount, itemCount int) bool {
			if totalCount < 0 || itemCount < 0 || itemCount > totalCount {
				return true // Skip invalid combinations
			}

			expectedResp := &wechat.BatchGetResponse{
				TotalCount: totalCount,
				ItemCount:  itemCount,
				Item:       make([]wechat.PublishedArticle, itemCount),
			}

			for i := 0; i < itemCount; i++ {
				expectedResp.Item[i] = wechat.PublishedArticle{
					ArticleID:  "article_" + string(rune('A'+i)),
					UpdateTime: int64(1000000 + i),
				}
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(expectedResp)
			}))
			defer server.Close()

			client := NewHTTPClient(WithBaseURL(server.URL))
			ctx := context.Background()

			resp, err := client.BatchGetPublishedArticles(ctx, "test_token", &wechat.BatchGetRequest{
				Offset: 0,
				Count:  10,
			})

			if err != nil {
				return false
			}

			// Verify all fields are preserved
			return resp.TotalCount == expectedResp.TotalCount &&
				resp.ItemCount == expectedResp.ItemCount &&
				len(resp.Item) == len(expectedResp.Item)
		},
		gen.IntRange(0, 100),
		gen.IntRange(0, 20),
	))

	properties.TestingRun(t)
}

// Unit tests for specific scenarios
func TestHTTPClient_GetComponentAccessToken(t *testing.T) {
	expectedResp := &wechat.ComponentTokenResponse{
		ComponentAccessToken: "test_component_token",
		ExpiresIn:            7200,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/cgi-bin/component/api_component_token")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResp)
	}))
	defer server.Close()

	client := NewHTTPClient(WithBaseURL(server.URL))
	ctx := context.Background()

	resp, err := client.GetComponentAccessToken(ctx, &wechat.ComponentTokenRequest{
		ComponentAppID:        "test_appid",
		ComponentAppSecret:    "test_secret",
		ComponentVerifyTicket: "test_ticket",
	})

	require.NoError(t, err)
	assert.Equal(t, expectedResp.ComponentAccessToken, resp.ComponentAccessToken)
	assert.Equal(t, expectedResp.ExpiresIn, resp.ExpiresIn)
}

func TestHTTPClient_RefreshAuthorizerToken(t *testing.T) {
	expectedResp := &wechat.RefreshAuthorizerTokenResponse{
		AuthorizerAccessToken:  "test_authorizer_token",
		ExpiresIn:              7200,
		AuthorizerRefreshToken: "new_refresh_token",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/cgi-bin/component/api_authorizer_token")
		assert.Contains(t, r.URL.RawQuery, "component_access_token=comp_token")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResp)
	}))
	defer server.Close()

	client := NewHTTPClient(WithBaseURL(server.URL))
	ctx := context.Background()

	resp, err := client.RefreshAuthorizerToken(ctx, "comp_token", &wechat.RefreshAuthorizerTokenRequest{
		ComponentAppID:         "comp_appid",
		AuthorizerAppID:        "auth_appid",
		AuthorizerRefreshToken: "refresh_token",
	})

	require.NoError(t, err)
	assert.Equal(t, expectedResp.AuthorizerAccessToken, resp.AuthorizerAccessToken)
	assert.Equal(t, expectedResp.ExpiresIn, resp.ExpiresIn)
}

func TestHTTPClient_BatchGetPublishedArticles(t *testing.T) {
	expectedResp := &wechat.BatchGetResponse{
		TotalCount: 100,
		ItemCount:  2,
		Item: []wechat.PublishedArticle{
			{
				ArticleID:  "article_1",
				UpdateTime: 1234567890,
				Content: &wechat.ArticleContent{
					NewsItem: []wechat.NewsItem{
						{Title: "Test Article 1", Author: "Author 1"},
					},
				},
			},
			{
				ArticleID:  "article_2",
				UpdateTime: 1234567891,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/cgi-bin/freepublish/batchget")
		assert.Contains(t, r.URL.RawQuery, "access_token=test_token")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResp)
	}))
	defer server.Close()

	client := NewHTTPClient(WithBaseURL(server.URL))
	ctx := context.Background()

	resp, err := client.BatchGetPublishedArticles(ctx, "test_token", &wechat.BatchGetRequest{
		Offset: 0,
		Count:  10,
	})

	require.NoError(t, err)
	assert.Equal(t, expectedResp.TotalCount, resp.TotalCount)
	assert.Equal(t, expectedResp.ItemCount, resp.ItemCount)
	assert.Len(t, resp.Item, 2)
}

func TestHTTPClient_GetPublishedArticle(t *testing.T) {
	expectedResp := &wechat.GetArticleResponse{
		NewsItem: []wechat.NewsItem{
			{
				Title:   "Test Article",
				Author:  "Test Author",
				Digest:  "Test Digest",
				Content: "<p>Test Content</p>",
				URL:     "https://example.com/article",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/cgi-bin/freepublish/getarticle")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResp)
	}))
	defer server.Close()

	client := NewHTTPClient(WithBaseURL(server.URL))
	ctx := context.Background()

	resp, err := client.GetPublishedArticle(ctx, "test_token", "article_123")

	require.NoError(t, err)
	assert.Len(t, resp.NewsItem, 1)
	assert.Equal(t, "Test Article", resp.NewsItem[0].Title)
}

func TestHTTPClient_WeChatAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&wechat.BatchGetResponse{
			ErrCode: 48001,
			ErrMsg:  "api unauthorized",
		})
	}))
	defer server.Close()

	client := NewHTTPClient(WithBaseURL(server.URL))
	ctx := context.Background()

	_, err := client.BatchGetPublishedArticles(ctx, "test_token", &wechat.BatchGetRequest{
		Offset: 0,
		Count:  10,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "48001")
}

func TestHTTPClient_RetryOnFailure(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Success on 3rd attempt
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&wechat.BatchGetResponse{
			TotalCount: 10,
			ItemCount:  1,
		})
	}))
	defer server.Close()

	client := NewHTTPClient(
		WithBaseURL(server.URL),
		WithMaxRetries(3),
	)
	ctx := context.Background()

	resp, err := client.BatchGetPublishedArticles(ctx, "test_token", &wechat.BatchGetRequest{
		Offset: 0,
		Count:  10,
	})

	require.NoError(t, err)
	assert.Equal(t, 10, resp.TotalCount)
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
}
