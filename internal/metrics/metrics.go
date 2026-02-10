// Package metrics provides Prometheus metrics for the service.
package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metric collectors.
type Metrics struct {
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	GRPCRequestsTotal   *prometheus.CounterVec
	GRPCRequestDuration *prometheus.HistogramVec
	WeChatAPITotal      *prometheus.CounterVec
	WeChatAPIDuration   *prometheus.HistogramVec
	CacheHitsTotal      *prometheus.CounterVec
	CacheMissesTotal    *prometheus.CounterVec
}

// New creates and registers all Prometheus metrics.
func New() *Metrics {
	m := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		GRPCRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "grpc_requests_total",
				Help: "Total number of gRPC requests",
			},
			[]string{"method", "code"},
		),
		GRPCRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "grpc_request_duration_seconds",
				Help:    "gRPC request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		WeChatAPITotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "wechat_api_requests_total",
				Help: "Total number of WeChat API requests",
			},
			[]string{"endpoint", "status"},
		),
		WeChatAPIDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "wechat_api_request_duration_seconds",
				Help:    "WeChat API request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"endpoint"},
		),
		CacheHitsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"key_type"},
		),
		CacheMissesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"key_type"},
		),
	}

	prometheus.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.GRPCRequestsTotal,
		m.GRPCRequestDuration,
		m.WeChatAPITotal,
		m.WeChatAPIDuration,
		m.CacheHitsTotal,
		m.CacheMissesTotal,
	)

	return m
}

// GinMiddleware returns a Gin middleware that records HTTP metrics.
func (m *Metrics) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		m.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
