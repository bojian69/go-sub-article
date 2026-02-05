package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/service"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// MockArticleService is a mock implementation of ArticleService
type MockArticleService struct {
	batchGetResp   *service.BatchGetArticlesResponse
	getArticleResp *service.GetArticleResponse
	err            error
}

func (m *MockArticleService) BatchGetPublishedArticles(ctx context.Context, req *service.BatchGetArticlesRequest) (*service.BatchGetArticlesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.batchGetResp, nil
}

func (m *MockArticleService) GetPublishedArticle(ctx context.Context, req *service.GetArticleRequest) (*service.GetArticleResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.getArticleResp, nil
}

// Property 6: Request Parameter Validation
// For any request with invalid parameters, the handler SHALL reject with validation error.
// **Validates: Requirements 2.1, 2.2, 2.4, 3.1, 3.2**
func TestProperty_RequestParameterValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: Invalid count returns 400
	properties.Property("invalid count returns 400", prop.ForAll(
		func(count int) bool {
			// Only test invalid counts
			if count >= 1 && count <= 20 {
				return true
			}

			mockSvc := &MockArticleService{
				batchGetResp: &service.BatchGetArticlesResponse{},
			}
			handler := NewHandler(mockSvc, slog.Default())

			r := gin.New()
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/accounts/test_appid/articles?count=%d", count), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.IntRange(-10, 30),
	))

	// Property: Negative offset returns 400
	properties.Property("negative offset returns 400", prop.ForAll(
		func(offset int) bool {
			if offset >= 0 {
				return true
			}

			mockSvc := &MockArticleService{
				batchGetResp: &service.BatchGetArticlesResponse{},
			}
			handler := NewHandler(mockSvc, slog.Default())

			r := gin.New()
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/accounts/test_appid/articles?offset=%d", offset), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.IntRange(-100, 10),
	))

	properties.TestingRun(t)
}

// Property 11: HTTP Response Structure
// For any HTTP response, the body SHALL include code, message, and request_id fields.
// **Validates: Requirements 4.3, 4.4**
func TestProperty_HTTPResponseStructure(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: Success response has required fields
	properties.Property("success response has code, message, request_id", prop.ForAll(
		func(totalCount, itemCount int) bool {
			if totalCount < 0 || itemCount < 0 {
				return true
			}

			mockSvc := &MockArticleService{
				batchGetResp: &service.BatchGetArticlesResponse{
					TotalCount: totalCount,
					ItemCount:  itemCount,
				},
			}
			handler := NewHandler(mockSvc, slog.Default())

			r := gin.New()
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodGet, "/v1/accounts/test_appid/articles?count=10", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp StandardResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				return false
			}

			// Check required fields
			return resp.RequestID != "" && resp.Message != ""
		},
		gen.IntRange(0, 1000),
		gen.IntRange(0, 20),
	))

	// Property: Error response has required fields
	properties.Property("error response has code, message, request_id", prop.ForAll(
		func(count int) bool {
			// Only test invalid counts
			if count >= 1 && count <= 20 {
				return true
			}

			mockSvc := &MockArticleService{}
			handler := NewHandler(mockSvc, slog.Default())

			r := gin.New()
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/accounts/test_appid/articles?count=%d", count), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp StandardResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				return false
			}

			// Check required fields
			return resp.RequestID != "" && resp.Message != "" && resp.Code != 0
		},
		gen.IntRange(-10, 30),
	))

	properties.TestingRun(t)
}

// Property 12: Request ID Uniqueness
// For any two distinct requests, the generated request_id values SHALL be different.
// **Validates: Requirements 4.5, 5.5**
func TestProperty_RequestIDUniqueness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: Multiple requests have unique request_ids
	properties.Property("multiple requests have unique request_ids", prop.ForAll(
		func(numRequests int) bool {
			if numRequests < 2 || numRequests > 50 {
				return true
			}

			mockSvc := &MockArticleService{
				batchGetResp: &service.BatchGetArticlesResponse{},
			}
			handler := NewHandler(mockSvc, slog.Default())

			r := gin.New()
			handler.RegisterRoutes(r)

			requestIDs := make(map[string]bool)

			for i := 0; i < numRequests; i++ {
				req := httptest.NewRequest(http.MethodGet, "/v1/accounts/test_appid/articles?count=10", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				var resp StandardResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					return false
				}

				if requestIDs[resp.RequestID] {
					return false // Duplicate found
				}
				requestIDs[resp.RequestID] = true
			}

			return true
		},
		gen.IntRange(2, 20),
	))

	properties.TestingRun(t)
}

// Unit tests
func TestHandler_BatchGetArticles_Success(t *testing.T) {
	mockSvc := &MockArticleService{
		batchGetResp: &service.BatchGetArticlesResponse{
			TotalCount: 100,
			ItemCount:  2,
			Item: []wechat.PublishedArticle{
				{ArticleID: "article_1"},
				{ArticleID: "article_2"},
			},
		},
	}

	handler := NewHandler(mockSvc, slog.Default())
	r := gin.New()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/test_appid/articles?offset=0&count=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp StandardResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, CodeSuccess, resp.Code)
	assert.Equal(t, "success", resp.Message)
	assert.NotEmpty(t, resp.RequestID)
	assert.NotNil(t, resp.Data)
}

func TestHandler_BatchGetArticles_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "count too small",
			url:  "/v1/accounts/test_appid/articles?count=0",
		},
		{
			name: "count too large",
			url:  "/v1/accounts/test_appid/articles?count=21",
		},
		{
			name: "negative offset",
			url:  "/v1/accounts/test_appid/articles?offset=-1&count=10",
		},
		{
			name: "invalid no_content",
			url:  "/v1/accounts/test_appid/articles?count=10&no_content=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockArticleService{}
			handler := NewHandler(mockSvc, slog.Default())
			r := gin.New()
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var resp StandardResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			assert.Equal(t, CodeInvalidParam, resp.Code)
			assert.NotEmpty(t, resp.RequestID)
		})
	}
}

func TestHandler_GetArticle_Success(t *testing.T) {
	mockSvc := &MockArticleService{
		getArticleResp: &service.GetArticleResponse{
			NewsItem: []wechat.NewsItem{
				{Title: "Test Article", Author: "Test Author"},
			},
		},
	}

	handler := NewHandler(mockSvc, slog.Default())
	r := gin.New()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/test_appid/articles/article_123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp StandardResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, CodeSuccess, resp.Code)
	assert.NotEmpty(t, resp.RequestID)
}

func TestHandler_ServiceError(t *testing.T) {
	mockSvc := &MockArticleService{
		err: assert.AnError,
	}

	handler := NewHandler(mockSvc, slog.Default())
	r := gin.New()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/test_appid/articles?count=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp StandardResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, CodeInternalErr, resp.Code)
	assert.NotEmpty(t, resp.RequestID)
}

func TestGenerateRequestID(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateRequestID()
		assert.NotEmpty(t, id)
		assert.False(t, ids[id], "duplicate request ID generated")
		ids[id] = true
	}
}
