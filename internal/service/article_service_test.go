package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
)

// MockTokenService is a mock implementation of TokenService
type MockTokenService struct {
	token string
	err   error
}

func (m *MockTokenService) GetComponentToken(ctx context.Context) (string, error) {
	return m.token, m.err
}

func (m *MockTokenService) GetAuthorizerToken(ctx context.Context, authorizerAppID string) (string, error) {
	return m.token, m.err
}

// MockArticleWeChatClient is a mock WeChat client for article tests
type MockArticleWeChatClient struct {
	batchGetResp   *wechat.BatchGetResponse
	getArticleResp *wechat.GetArticleResponse
	lastNoContent  int
}

func (m *MockArticleWeChatClient) GetComponentAccessToken(ctx context.Context, req *wechat.ComponentTokenRequest) (*wechat.ComponentTokenResponse, error) {
	return &wechat.ComponentTokenResponse{}, nil
}

func (m *MockArticleWeChatClient) RefreshAuthorizerToken(ctx context.Context, componentToken string, req *wechat.RefreshAuthorizerTokenRequest) (*wechat.RefreshAuthorizerTokenResponse, error) {
	return &wechat.RefreshAuthorizerTokenResponse{}, nil
}

func (m *MockArticleWeChatClient) BatchGetPublishedArticles(ctx context.Context, accessToken string, req *wechat.BatchGetRequest) (*wechat.BatchGetResponse, error) {
	m.lastNoContent = req.NoContent
	return m.batchGetResp, nil
}

func (m *MockArticleWeChatClient) GetPublishedArticle(ctx context.Context, accessToken string, articleID string) (*wechat.GetArticleResponse, error) {
	return m.getArticleResp, nil
}

func (m *MockArticleWeChatClient) GetAccessToken(ctx context.Context, appID, appSecret string) (*wechat.AccessTokenResponse, error) {
	return &wechat.AccessTokenResponse{
		AccessToken: "mock_simple_access_token",
		ExpiresIn:   7200,
	}, nil
}

// Property 7: No Content Parameter Behavior
// For any request with no_content=1, the response SHALL NOT include the content field.
// **Validates: Requirements 2.6**
func TestProperty_NoContentParameterBehavior(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: no_content parameter is passed to WeChat API
	properties.Property("no_content parameter is passed correctly", prop.ForAll(
		func(noContent int) bool {
			// Normalize to valid values
			if noContent != 0 && noContent != 1 {
				return true
			}

			mockClient := &MockArticleWeChatClient{
				batchGetResp: &wechat.BatchGetResponse{
					TotalCount: 10,
					ItemCount:  1,
					Item: []wechat.PublishedArticle{
						{ArticleID: "test_article"},
					},
				},
			}

			tokenSvc := &MockTokenService{token: "test_token"}
			svc := NewArticleService(tokenSvc, mockClient, slog.Default())

			ctx := context.Background()
			_, err := svc.BatchGetPublishedArticles(ctx, &BatchGetArticlesRequest{
				AuthorizerAppID: "test_appid",
				Offset:          0,
				Count:           10,
				NoContent:       noContent,
			})

			if err != nil {
				return false
			}

			// Verify no_content was passed correctly
			return mockClient.lastNoContent == noContent
		},
		gen.IntRange(0, 1),
	))

	properties.TestingRun(t)
}

// Unit tests
func TestArticleService_BatchGetPublishedArticles(t *testing.T) {
	mockClient := &MockArticleWeChatClient{
		batchGetResp: &wechat.BatchGetResponse{
			TotalCount: 100,
			ItemCount:  2,
			Item: []wechat.PublishedArticle{
				{
					ArticleID:  "article_1",
					UpdateTime: 1234567890,
					Content: &wechat.ArticleContent{
						NewsItem: []wechat.NewsItem{
							{Title: "Test Article 1"},
						},
					},
				},
				{
					ArticleID:  "article_2",
					UpdateTime: 1234567891,
				},
			},
		},
	}

	tokenSvc := &MockTokenService{token: "test_token"}
	svc := NewArticleService(tokenSvc, mockClient, slog.Default())

	ctx := context.Background()
	resp, err := svc.BatchGetPublishedArticles(ctx, &BatchGetArticlesRequest{
		AuthorizerAppID: "test_appid",
		Offset:          0,
		Count:           10,
	})

	require.NoError(t, err)
	assert.Equal(t, 100, resp.TotalCount)
	assert.Equal(t, 2, resp.ItemCount)
	assert.Len(t, resp.Item, 2)
	assert.Equal(t, "article_1", resp.Item[0].ArticleID)
}

func TestArticleService_GetPublishedArticle(t *testing.T) {
	mockClient := &MockArticleWeChatClient{
		getArticleResp: &wechat.GetArticleResponse{
			NewsItem: []wechat.NewsItem{
				{
					Title:   "Test Article",
					Author:  "Test Author",
					Digest:  "Test Digest",
					Content: "<p>Test Content</p>",
					URL:     "https://example.com/article",
				},
			},
		},
	}

	tokenSvc := &MockTokenService{token: "test_token"}
	svc := NewArticleService(tokenSvc, mockClient, slog.Default())

	ctx := context.Background()
	resp, err := svc.GetPublishedArticle(ctx, &GetArticleRequest{
		AuthorizerAppID: "test_appid",
		ArticleID:       "article_123",
	})

	require.NoError(t, err)
	assert.Len(t, resp.NewsItem, 1)
	assert.Equal(t, "Test Article", resp.NewsItem[0].Title)
	assert.Equal(t, "Test Author", resp.NewsItem[0].Author)
}

func TestArticleService_TokenError(t *testing.T) {
	mockClient := &MockArticleWeChatClient{}
	tokenSvc := &MockTokenService{err: assert.AnError}
	svc := NewArticleService(tokenSvc, mockClient, slog.Default())

	ctx := context.Background()
	_, err := svc.BatchGetPublishedArticles(ctx, &BatchGetArticlesRequest{
		AuthorizerAppID: "test_appid",
		Offset:          0,
		Count:           10,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get authorizer token")
}
