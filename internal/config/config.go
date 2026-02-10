// Package config provides configuration loading and validation for the WeChat subscription service.
package config

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// Config represents the root configuration structure.
type Config struct {
	Log    LogConfig    `mapstructure:"log"`
	Server ServerConfig `mapstructure:"server" validate:"required"`
	Redis  RedisConfig  `mapstructure:"redis" validate:"required"`
	WeChat WeChatConfig `mapstructure:"wechat" validate:"required"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level   string        `mapstructure:"level"`   // debug, info, warn, error
	Output  string        `mapstructure:"output"`  // console, file, both
	Service string        `mapstructure:"service"` // service name
	File    LogFileConfig `mapstructure:"file"`
}

// LogFileConfig holds file logging configuration.
type LogFileConfig struct {
	Path     string `mapstructure:"path"`     // log directory
	Filename string `mapstructure:"filename"` // log file name
	MaxAge   int    `mapstructure:"max_age"`  // max days to retain
	Compress bool   `mapstructure:"compress"` // compress rotated files
}

// ServerConfig holds HTTP and gRPC server configuration.
type ServerConfig struct {
	HTTPPort int `mapstructure:"http_port" validate:"required,min=1,max=65535"`
	GRPCPort int `mapstructure:"grpc_port" validate:"required,min=1,max=65535"`
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	Host     string `mapstructure:"host" validate:"required"`
	Port     int    `mapstructure:"port" validate:"required,min=1,max=65535"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db" validate:"min=0,max=15"`
}

// Addr returns the Redis address in host:port format.
func (r *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// WeChatConfig holds WeChat third-party platform configuration.
type WeChatConfig struct {
	SimpleMode  SimpleModeConfig   `mapstructure:"simple_mode"`
	Component   ComponentConfig    `mapstructure:"component"`
	Authorizers []AuthorizerConfig `mapstructure:"authorizers"`
}

// SimpleModeConfig holds simple mode configuration (direct access_token).
type SimpleModeConfig struct {
	Enabled  bool            `mapstructure:"enabled"`
	Accounts []SimpleAccount `mapstructure:"accounts"`
}

// SimpleAccount holds simple mode account credentials.
type SimpleAccount struct {
	AppID     string `mapstructure:"app_id"`
	AppSecret string `mapstructure:"app_secret"`
}

// ComponentConfig holds third-party platform credentials.
type ComponentConfig struct {
	AppID        string `mapstructure:"app_id"`
	AppSecret    string `mapstructure:"app_secret"`
	VerifyTicket string `mapstructure:"verify_ticket"`
}

// AuthorizerConfig holds official account authorization information.
type AuthorizerConfig struct {
	AppID        string `mapstructure:"app_id"`
	RefreshToken string `mapstructure:"refresh_token"`
}

// IsSimpleMode returns true if simple mode is enabled.
func (w *WeChatConfig) IsSimpleMode() bool {
	return w.SimpleMode.Enabled && len(w.SimpleMode.Accounts) > 0
}

// GetSimpleAccountByAppID returns the simple account config for the given appid.
func (w *WeChatConfig) GetSimpleAccountByAppID(appID string) (*SimpleAccount, bool) {
	for i := range w.SimpleMode.Accounts {
		if w.SimpleMode.Accounts[i].AppID == appID {
			return &w.SimpleMode.Accounts[i], true
		}
	}
	return nil, false
}

// GetAuthorizerByAppID returns the authorizer config for the given appid.
func (w *WeChatConfig) GetAuthorizerByAppID(appID string) (*AuthorizerConfig, bool) {
	for i := range w.Authorizers {
		if w.Authorizers[i].AppID == appID {
			return &w.Authorizers[i], true
		}
	}
	return nil, false
}

// Load loads configuration from the specified path.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Support environment variable overrides
	v.SetEnvPrefix("WECHAT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadFromEnv loads configuration with default path based on environment.
func LoadFromEnv(env string) (*Config, error) {
	configPath := fmt.Sprintf("configs/config.%s.yaml", env)
	return Load(configPath)
}

// Validate validates the configuration using struct tags.
func Validate(cfg *Config) error {
	validate := validator.New()

	if err := validate.Struct(cfg); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			var errMsgs []string
			for _, e := range validationErrors {
				errMsgs = append(errMsgs, fmt.Sprintf("field '%s' failed validation: %s", e.Field(), e.Tag()))
			}
			return fmt.Errorf("configuration validation failed: %s", strings.Join(errMsgs, "; "))
		}
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Additional business validation
	if cfg.Server.HTTPPort == cfg.Server.GRPCPort {
		return fmt.Errorf("HTTP port and gRPC port cannot be the same")
	}

	// Validate WeChat config based on mode
	if cfg.WeChat.IsSimpleMode() {
		// Simple mode validation
		for i, acc := range cfg.WeChat.SimpleMode.Accounts {
			if acc.AppID == "" {
				return fmt.Errorf("simple_mode.accounts[%d].app_id is required", i)
			}
			if acc.AppSecret == "" {
				return fmt.Errorf("simple_mode.accounts[%d].app_secret is required", i)
			}
		}
	} else {
		// Third-party platform mode validation
		if cfg.WeChat.Component.AppID == "" {
			return fmt.Errorf("wechat.component.app_id is required when simple_mode is disabled")
		}
		if cfg.WeChat.Component.AppSecret == "" {
			return fmt.Errorf("wechat.component.app_secret is required when simple_mode is disabled")
		}
		if cfg.WeChat.Component.VerifyTicket == "" {
			return fmt.Errorf("wechat.component.verify_ticket is required when simple_mode is disabled")
		}
		if len(cfg.WeChat.Authorizers) == 0 {
			return fmt.Errorf("wechat.authorizers is required when simple_mode is disabled")
		}
		for i, auth := range cfg.WeChat.Authorizers {
			if auth.AppID == "" {
				return fmt.Errorf("wechat.authorizers[%d].app_id is required", i)
			}
			if auth.RefreshToken == "" {
				return fmt.Errorf("wechat.authorizers[%d].refresh_token is required", i)
			}
		}
	}

	return nil
}
