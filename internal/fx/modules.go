// Package fx provides Uber FX dependency injection modules.
package fx

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "git.uhomes.net/uhs-go/wechat-subscription-svc/api/proto"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/config"
	grpchandler "git.uhomes.net/uhs-go/wechat-subscription-svc/internal/handler/grpc"
	httphandler "git.uhomes.net/uhs-go/wechat-subscription-svc/internal/handler/http"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/logger"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/metrics"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/repository/cache"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/service"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/wechat/client"
)

// ConfigModule provides configuration.
var ConfigModule = fx.Module("config",
	fx.Provide(func() (*config.Config, error) {
		env := os.Getenv("APP_ENV")
		if env == "" {
			env = "local"
		}
		return config.LoadFromEnv(env)
	}),
)

// LoggerModule provides logging.
var LoggerModule = fx.Module("logger",
	fx.Provide(func(cfg *config.Config) (*logger.Logger, error) {
		logCfg := &logger.Config{
			Level:   cfg.Log.Level,
			Output:  cfg.Log.Output,
			Service: cfg.Log.Service,
			File: logger.FileConfig{
				Path:     cfg.Log.File.Path,
				Filename: cfg.Log.File.Filename,
				MaxAge:   cfg.Log.File.MaxAge,
				Compress: cfg.Log.File.Compress,
			},
		}

		// Set defaults if not configured
		if logCfg.Level == "" {
			logCfg.Level = "info"
		}
		if logCfg.Output == "" {
			logCfg.Output = "console"
		}

		return logger.New(logCfg)
	}),
	fx.Provide(func(l *logger.Logger) *slog.Logger {
		return l.Logger
	}),
)

// CacheModule provides Redis cache repository.
var CacheModule = fx.Module("cache",
	fx.Provide(func(cfg *config.Config) (cache.Repository, error) {
		return cache.NewRedisRepository(
			cfg.Redis.Addr(),
			cfg.Redis.Username,
			cfg.Redis.Password,
			cfg.Redis.DB,
		)
	}),
)

// WeChatModule provides WeChat client with circuit breaker.
var WeChatModule = fx.Module("wechat",
	fx.Provide(func(logger *slog.Logger) client.Client {
		httpClient := client.NewHTTPClient(
			client.WithLogger(logger),
		)
		return client.NewCircuitBreakerClient(httpClient, logger)
	}),
)

// ServiceModule provides business services.
var ServiceModule = fx.Module("service",
	fx.Provide(func(cfg *config.Config, cacheRepo cache.Repository, wechatClient client.Client, logger *slog.Logger) service.TokenService {
		return service.NewTokenService(&cfg.WeChat, cacheRepo, wechatClient, logger)
	}),
	fx.Provide(func(tokenSvc service.TokenService, wechatClient client.Client, logger *slog.Logger) service.ArticleService {
		return service.NewArticleService(tokenSvc, wechatClient, logger)
	}),
)

// HandlerModule provides HTTP and gRPC handlers.
var HandlerModule = fx.Module("handler",
	fx.Provide(func(articleSvc service.ArticleService, cacheRepo cache.Repository, logger *slog.Logger) *httphandler.Handler {
		return httphandler.NewHandler(articleSvc, cacheRepo, logger)
	}),
	fx.Provide(func(articleSvc service.ArticleService, logger *slog.Logger) *grpchandler.Handler {
		return grpchandler.NewHandler(articleSvc, logger)
	}),
)

// MetricsModule provides Prometheus metrics.
var MetricsModule = fx.Module("metrics",
	fx.Provide(metrics.New),
)

// HTTPServerModule provides HTTP server.
var HTTPServerModule = fx.Module("http_server",
	fx.Provide(func(handler *httphandler.Handler, m *metrics.Metrics, logger *slog.Logger) *gin.Engine {
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		r.Use(gin.Recovery())
		r.Use(requestLoggingMiddleware(logger))
		r.Use(m.GinMiddleware())
		r.Use(timeoutMiddleware(30 * time.Second))
		r.GET("/metrics", metrics.Handler())
		handler.RegisterRoutes(r)
		return r
	}),
	fx.Invoke(func(lc fx.Lifecycle, cfg *config.Config, r *gin.Engine, logger *slog.Logger) {
		srv := &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Server.HTTPPort),
			Handler: r,
		}

		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				ln, err := net.Listen("tcp", srv.Addr)
				if err != nil {
					return err
				}
				logger.Info("HTTP server starting", slog.String("addr", srv.Addr))
				go srv.Serve(ln)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				logger.Info("HTTP server stopping")
				return srv.Shutdown(ctx)
			},
		})
	}),
)

// requestLoggingMiddleware logs each HTTP request with method, path, status, and latency.
func requestLoggingMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", statusCode),
			slog.Duration("latency", latency),
			slog.String("client_ip", c.ClientIP()),
		}
		if query != "" {
			attrs = append(attrs, slog.String("query", query))
		}

		if statusCode >= 500 {
			logger.Error("[HTTP] request", attrs...)
		} else if statusCode >= 400 {
			logger.Warn("[HTTP] request", attrs...)
		} else {
			logger.Info("[HTTP] request", attrs...)
		}
	}
}

// timeoutMiddleware adds a timeout to each request context.
func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GRPCServerModule provides gRPC server.
var GRPCServerModule = fx.Module("grpc_server",
	fx.Provide(func(handler *grpchandler.Handler, m *metrics.Metrics, logger *slog.Logger) *grpc.Server {
		srv := grpc.NewServer(
			grpc.ChainUnaryInterceptor(
				grpcRecoveryInterceptor(logger),
				grpcLoggingInterceptor(logger),
				grpcMetricsInterceptor(m),
			),
		)
		pb.RegisterSubscriptionServiceServer(srv, handler)
		return srv
	}),
	fx.Invoke(func(lc fx.Lifecycle, cfg *config.Config, srv *grpc.Server, logger *slog.Logger) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				addr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
				ln, err := net.Listen("tcp", addr)
				if err != nil {
					return err
				}
				logger.Info("gRPC server starting", slog.String("addr", addr))
				go srv.Serve(ln)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				logger.Info("gRPC server stopping")
				srv.GracefulStop()
				return nil
			},
		})
	}),
)

// grpcRecoveryInterceptor recovers from panics in gRPC handlers.
func grpcRecoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[gRPC] panic recovered",
					slog.String("method", info.FullMethod),
					slog.Any("panic", r),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// grpcLoggingInterceptor logs each gRPC request with method, status, and latency.
func grpcLoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		latency := time.Since(start)

		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			}
		}

		attrs := []any{
			slog.String("method", info.FullMethod),
			slog.String("code", code.String()),
			slog.Duration("latency", latency),
		}

		if err != nil {
			logger.Warn("[gRPC] request", append(attrs, slog.String("error", err.Error()))...)
		} else {
			logger.Info("[gRPC] request", attrs...)
		}

		return resp, err
	}
}

// grpcMetricsInterceptor records gRPC request metrics.
func grpcMetricsInterceptor(m *metrics.Metrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start).Seconds()

		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			}
		}

		m.GRPCRequestsTotal.WithLabelValues(info.FullMethod, code.String()).Inc()
		m.GRPCRequestDuration.WithLabelValues(info.FullMethod).Observe(duration)

		return resp, err
	}
}

// AllModules combines all modules.
var AllModules = fx.Options(
	ConfigModule,
	LoggerModule,
	CacheModule,
	WeChatModule,
	MetricsModule,
	ServiceModule,
	HandlerModule,
	HTTPServerModule,
	GRPCServerModule,
)
