// Package fx provides Uber FX dependency injection modules.
package fx

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"google.golang.org/grpc"

	pb "git.uhomes.net/uhs-go/wechat-subscription-svc/api/proto"
	"git.uhomes.net/uhs-go/wechat-subscription-svc/internal/config"
	grpchandler "git.uhomes.net/uhs-go/wechat-subscription-svc/internal/handler/grpc"
	httphandler "git.uhomes.net/uhs-go/wechat-subscription-svc/internal/handler/http"
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
	fx.Provide(func() *slog.Logger {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}),
)

// CacheModule provides Redis cache repository.
var CacheModule = fx.Module("cache",
	fx.Provide(func(cfg *config.Config) (cache.Repository, error) {
		return cache.NewRedisRepository(
			cfg.Redis.Addr(),
			cfg.Redis.Password,
			cfg.Redis.DB,
		)
	}),
)

// WeChatModule provides WeChat client.
var WeChatModule = fx.Module("wechat",
	fx.Provide(func(logger *slog.Logger) client.Client {
		return client.NewHTTPClient(
			client.WithLogger(logger),
		)
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
	fx.Provide(func(articleSvc service.ArticleService, logger *slog.Logger) *httphandler.Handler {
		return httphandler.NewHandler(articleSvc, logger)
	}),
	fx.Provide(func(articleSvc service.ArticleService, logger *slog.Logger) *grpchandler.Handler {
		return grpchandler.NewHandler(articleSvc, logger)
	}),
)

// HTTPServerModule provides HTTP server.
var HTTPServerModule = fx.Module("http_server",
	fx.Provide(func(handler *httphandler.Handler, logger *slog.Logger) *gin.Engine {
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		r.Use(gin.Recovery())
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

// GRPCServerModule provides gRPC server.
var GRPCServerModule = fx.Module("grpc_server",
	fx.Provide(func(handler *grpchandler.Handler) *grpc.Server {
		srv := grpc.NewServer()
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

// AllModules combines all modules.
var AllModules = fx.Options(
	ConfigModule,
	LoggerModule,
	CacheModule,
	WeChatModule,
	ServiceModule,
	HandlerModule,
	HTTPServerModule,
	GRPCServerModule,
)
