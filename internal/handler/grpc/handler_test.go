package grpc

import (
	"context"
	"log/slog"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "git.uhomes.net/uhs-go/wechat-subscription-svc/api/proto"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/service"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat"
)

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

// Property 13: gRPC Status Code Mapping
// For any error condition, the gRPC handler SHALL return an appropriate gRPC status code.
// **Validates: Requirements 5.4**
func TestProperty_GRPCStatusCodeMapping(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: Missing authorizer_appid returns InvalidArgument
	properties.Property("missing authorizer_appid returns InvalidArgument", prop.ForAll(
		func(offset, count int32) bool {
			mockSvc := &MockArticleService{}
			handler := NewHandler(mockSvc, slog.Default())

			_, err := handler.BatchGetPublishedArticles(context.Background(), &pb.BatchGetArticlesRequest{
				AuthorizerAppid: "", // Missing
				Offset:          offset,
				Count:           count,
			})

			if err == nil {
				return false
			}

			st, ok := status.FromError(err)
			return ok && st.Code() == codes.InvalidArgument
		},
		gen.Int32Range(0, 100),
		gen.Int32Range(1, 20),
	))

	// Property: Invalid count returns InvalidArgument
	properties.Property("invalid count returns InvalidArgument", prop.ForAll(
		func(count int32) bool {
			// Only test invalid counts
			if count >= 1 && count <= 20 {
				return true
			}

			mockSvc := &MockArticleService{}
			handler := NewHandler(mockSvc, slog.Default())

			_, err := handler.BatchGetPublishedArticles(context.Background(), &pb.BatchGetArticlesRequest{
				AuthorizerAppid: "test_appid",
				Offset:          0,
				Count:           count,
			})

			if err == nil {
				return false
			}

			st, ok := status.FromError(err)
			return ok && st.Code() == codes.InvalidArgument
		},
		gen.Int32Range(-10, 30),
	))

	// Property: Service error returns Internal
	properties.Property("service error returns Internal", prop.ForAll(
		func(appID string) bool {
			if appID == "" {
				return true
			}

			mockSvc := &MockArticleService{
				err: assert.AnError,
			}
			handler := NewHandler(mockSvc, slog.Default())

			_, err := handler.BatchGetPublishedArticles(context.Background(), &pb.BatchGetArticlesRequest{
				AuthorizerAppid: appID,
				Offset:          0,
				Count:           10,
			})

			if err == nil {
				return false
			}

			st, ok := status.FromError(err)
			return ok && st.Code() == codes.Internal
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
	))

	properties.TestingRun(t)
}

// Unit tests
func TestHandler_BatchGetPublishedArticles_Success(t *testing.T) {
	mockSvc := &MockArticleService{
		batchGetResp: &service.BatchGetArticlesResponse{
			TotalCount: 100,
			ItemCount:  2,
			Item: []wechat.PublishedArticle{
				{ArticleID: "article_1", UpdateTime: 1234567890},
				{ArticleID: "article_2", UpdateTime: 1234567891},
			},
		},
	}

	handler := NewHandler(mockSvc, slog.Default())
	ctx := context.Background()

	resp, err := handler.BatchGetPublishedArticles(ctx, &pb.BatchGetArticlesRequest{
		AuthorizerAppid: "test_appid",
		Offset:          0,
		Count:           10,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(100), resp.TotalCount)
	assert.Equal(t, int32(2), resp.ItemCount)
	assert.Len(t, resp.Item, 2)
}

func TestHandler_BatchGetPublishedArticles_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		req     *pb.BatchGetArticlesRequest
		errCode codes.Code
	}{
		{
			name: "missing authorizer_appid",
			req: &pb.BatchGetArticlesRequest{
				AuthorizerAppid: "",
				Offset:          0,
				Count:           10,
			},
			errCode: codes.InvalidArgument,
		},
		{
			name: "negative offset",
			req: &pb.BatchGetArticlesRequest{
				AuthorizerAppid: "test_appid",
				Offset:          -1,
				Count:           10,
			},
			errCode: codes.InvalidArgument,
		},
		{
			name: "count too small",
			req: &pb.BatchGetArticlesRequest{
				AuthorizerAppid: "test_appid",
				Offset:          0,
				Count:           0,
			},
			errCode: codes.InvalidArgument,
		},
		{
			name: "count too large",
			req: &pb.BatchGetArticlesRequest{
				AuthorizerAppid: "test_appid",
				Offset:          0,
				Count:           21,
			},
			errCode: codes.InvalidArgument,
		},
		{
			name: "invalid no_content",
			req: &pb.BatchGetArticlesRequest{
				AuthorizerAppid: "test_appid",
				Offset:          0,
				Count:           10,
				NoContent:       2,
			},
			errCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockArticleService{}
			handler := NewHandler(mockSvc, slog.Default())

			_, err := handler.BatchGetPublishedArticles(context.Background(), tt.req)

			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.errCode, st.Code())
		})
	}
}

func TestHandler_GetPublishedArticle_Success(t *testing.T) {
	mockSvc := &MockArticleService{
		getArticleResp: &service.GetArticleResponse{
			NewsItem: []wechat.NewsItem{
				{
					Title:   "Test Article",
					Author:  "Test Author",
					Content: "<p>Test Content</p>",
				},
			},
		},
	}

	handler := NewHandler(mockSvc, slog.Default())
	ctx := context.Background()

	resp, err := handler.GetPublishedArticle(ctx, &pb.GetArticleRequest{
		AuthorizerAppid: "test_appid",
		ArticleId:       "article_123",
	})

	require.NoError(t, err)
	assert.Len(t, resp.NewsItem, 1)
	assert.Equal(t, "Test Article", resp.NewsItem[0].Title)
}

func TestHandler_GetPublishedArticle_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		req     *pb.GetArticleRequest
		errCode codes.Code
	}{
		{
			name: "missing authorizer_appid",
			req: &pb.GetArticleRequest{
				AuthorizerAppid: "",
				ArticleId:       "article_123",
			},
			errCode: codes.InvalidArgument,
		},
		{
			name: "missing article_id",
			req: &pb.GetArticleRequest{
				AuthorizerAppid: "test_appid",
				ArticleId:       "",
			},
			errCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockArticleService{}
			handler := NewHandler(mockSvc, slog.Default())

			_, err := handler.GetPublishedArticle(context.Background(), tt.req)

			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.errCode, st.Code())
		})
	}
}

func TestHandler_ServiceError(t *testing.T) {
	mockSvc := &MockArticleService{
		err: assert.AnError,
	}

	handler := NewHandler(mockSvc, slog.Default())
	ctx := context.Background()

	_, err := handler.BatchGetPublishedArticles(ctx, &pb.BatchGetArticlesRequest{
		AuthorizerAppid: "test_appid",
		Offset:          0,
		Count:           10,
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}
