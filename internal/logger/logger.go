// Package logger provides structured logging with file rotation support.
// This package is designed to be compatible with gox/log interface.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config holds logger configuration.
type Config struct {
	Level   string     `yaml:"level"`   // debug, info, warn, error
	Output  string     `yaml:"output"`  // console, file, both
	File    FileConfig `yaml:"file"`
	Service string     `yaml:"service"` // service name for structured logs
}

// FileConfig holds file logging configuration.
type FileConfig struct {
	Path     string `yaml:"path"`     // log directory path
	Filename string `yaml:"filename"` // log file name
	MaxAge   int    `yaml:"max_age"`  // max days to retain old log files
	Compress bool   `yaml:"compress"` // compress rotated files
}

// Context keys for trace information
type contextKey string

// ContextKey is exported for use in other packages
type ContextKey = contextKey

const (
	traceIDKey   contextKey = "trace_id"
	spanIDKey    contextKey = "span_id"
	requestIDKey contextKey = "request_id"
)

// Logger wraps slog.Logger with additional functionality.
type Logger struct {
	*slog.Logger
	config  *Config
	writers []io.Writer
}

// New creates a new Logger instance.
func New(cfg *Config) (*Logger, error) {
	if cfg == nil {
		cfg = &Config{
			Level:  "info",
			Output: "console",
		}
	}

	// Parse log level
	level := parseLevel(cfg.Level)

	// Setup writers
	var writers []io.Writer

	switch cfg.Output {
	case "file":
		fw, err := newFileWriter(cfg)
		if err != nil {
			return nil, err
		}
		writers = append(writers, fw)
	case "both":
		writers = append(writers, os.Stdout)
		fw, err := newFileWriter(cfg)
		if err != nil {
			return nil, err
		}
		writers = append(writers, fw)
	default: // console
		writers = append(writers, os.Stdout)
	}

	// Create multi-writer
	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	// Create handler with custom options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize time format for ELK/Loki compatibility
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format(time.RFC3339Nano))
				}
			}
			return a
		},
	}

	handler := slog.NewJSONHandler(writer, opts)

	// Add service name if configured
	var logger *slog.Logger
	if cfg.Service != "" {
		logger = slog.New(handler).With(slog.String("service", cfg.Service))
	} else {
		logger = slog.New(handler)
	}

	return &Logger{
		Logger:  logger,
		config:  cfg,
		writers: writers,
	}, nil
}

// newFileWriter creates a lumberjack logger for daily rotation.
func newFileWriter(cfg *Config) (io.Writer, error) {
	// Ensure log directory exists
	if cfg.File.Path != "" {
		if err := os.MkdirAll(cfg.File.Path, 0755); err != nil {
			return nil, err
		}
	}

	filename := cfg.File.Filename
	if filename == "" {
		filename = "app.log"
	}

	// Use lumberjack for rotation
	// Note: lumberjack rotates by size, we use a wrapper for daily rotation
	return &dailyRotateWriter{
		path:     cfg.File.Path,
		filename: filename,
		maxAge:   cfg.File.MaxAge,
		compress: cfg.File.Compress,
	}, nil
}

// dailyRotateWriter implements daily log rotation.
type dailyRotateWriter struct {
	path     string
	filename string
	maxAge   int
	compress bool

	currentDate string
	writer      *lumberjack.Logger
}

func (w *dailyRotateWriter) Write(p []byte) (n int, err error) {
	today := time.Now().Format("2006-01-02")

	// Check if we need to rotate
	if w.currentDate != today || w.writer == nil {
		if w.writer != nil {
			w.writer.Close()
		}

		// Create new log file with date suffix
		logFile := filepath.Join(w.path, today+"-"+w.filename)

		w.writer = &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    500, // MB - large enough to not rotate by size within a day
			MaxBackups: 0,   // Keep all backups (controlled by MaxAge)
			MaxAge:     w.maxAge,
			Compress:   w.compress,
			LocalTime:  true,
		}
		w.currentDate = today
	}

	return w.writer.Write(p)
}

// parseLevel converts string level to slog.Level.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithContext returns a logger with trace information from context.
func (l *Logger) WithContext(ctx context.Context) *slog.Logger {
	attrs := make([]any, 0, 6)

	if traceID := GetTraceID(ctx); traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}
	if spanID := GetSpanID(ctx); spanID != "" {
		attrs = append(attrs, slog.String("span_id", spanID))
	}
	if requestID := GetRequestID(ctx); requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}

	if len(attrs) > 0 {
		return l.Logger.With(attrs...)
	}
	return l.Logger
}

// Context helper functions

// WithTraceID adds trace ID to context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// GetTraceID retrieves trace ID from context.
func GetTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

// WithSpanID adds span ID to context.
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, spanIDKey, spanID)
}

// GetSpanID retrieves span ID from context.
func GetSpanID(ctx context.Context) string {
	if id, ok := ctx.Value(spanIDKey).(string); ok {
		return id
	}
	return ""
}

// WithRequestID adds request ID to context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// Close closes all writers.
func (l *Logger) Close() error {
	for _, w := range l.writers {
		if closer, ok := w.(io.Closer); ok {
			closer.Close()
		}
	}
	return nil
}
