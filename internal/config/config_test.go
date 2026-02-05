package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Create a temporary config file
	content := `
server:
  http_port: 8080
  grpc_port: 9090
redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
wechat:
  component:
    app_id: "test_component_appid"
    app_secret: "test_component_secret"
    verify_ticket: "test_verify_ticket"
  authorizers:
    - app_id: "auth_appid_1"
      refresh_token: "refresh_token_1"
    - app_id: "auth_appid_2"
      refresh_token: "refresh_token_2"
`
	tmpFile := createTempConfigFile(t, content)
	defer os.Remove(tmpFile)

	cfg, err := Load(tmpFile)
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify server config
	assert.Equal(t, 8080, cfg.Server.HTTPPort)
	assert.Equal(t, 9090, cfg.Server.GRPCPort)

	// Verify redis config
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, 6379, cfg.Redis.Port)
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr())

	// Verify wechat config
	assert.Equal(t, "test_component_appid", cfg.WeChat.Component.AppID)
	assert.Equal(t, "test_component_secret", cfg.WeChat.Component.AppSecret)
	assert.Equal(t, "test_verify_ticket", cfg.WeChat.Component.VerifyTicket)

	// Verify authorizers
	assert.Len(t, cfg.WeChat.Authorizers, 2)
	assert.Equal(t, "auth_appid_1", cfg.WeChat.Authorizers[0].AppID)
	assert.Equal(t, "refresh_token_1", cfg.WeChat.Authorizers[0].RefreshToken)
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		errMsg  string
	}{
		{
			name: "missing server config",
			content: `
redis:
  host: localhost
  port: 6379
wechat:
  component:
    app_id: "test"
    app_secret: "test"
    verify_ticket: "test"
  authorizers:
    - app_id: "auth"
      refresh_token: "token"
`,
			errMsg: "HTTPPort",
		},
		{
			name: "missing redis host",
			content: `
server:
  http_port: 8080
  grpc_port: 9090
redis:
  port: 6379
wechat:
  component:
    app_id: "test"
    app_secret: "test"
    verify_ticket: "test"
  authorizers:
    - app_id: "auth"
      refresh_token: "token"
`,
			errMsg: "Host",
		},
		{
			name: "missing component app_id",
			content: `
server:
  http_port: 8080
  grpc_port: 9090
redis:
  host: localhost
  port: 6379
wechat:
  component:
    app_secret: "test"
    verify_ticket: "test"
  authorizers:
    - app_id: "auth"
      refresh_token: "token"
`,
			errMsg: "app_id",
		},
		{
			name: "empty authorizers",
			content: `
server:
  http_port: 8080
  grpc_port: 9090
redis:
  host: localhost
  port: 6379
wechat:
  component:
    app_id: "test"
    app_secret: "test"
    verify_ticket: "test"
  authorizers: []
`,
			errMsg: "authorizers",
		},
		{
			name: "authorizer missing refresh_token",
			content: `
server:
  http_port: 8080
  grpc_port: 9090
redis:
  host: localhost
  port: 6379
wechat:
  component:
    app_id: "test"
    app_secret: "test"
    verify_ticket: "test"
  authorizers:
    - app_id: "auth"
`,
			errMsg: "refresh_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempConfigFile(t, tt.content)
			defer os.Remove(tmpFile)

			cfg, err := Load(tmpFile)
			assert.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestLoad_MultipleAuthorizers(t *testing.T) {
	content := `
server:
  http_port: 8080
  grpc_port: 9090
redis:
  host: localhost
  port: 6379
wechat:
  component:
    app_id: "component"
    app_secret: "secret"
    verify_ticket: "ticket"
  authorizers:
    - app_id: "auth1"
      refresh_token: "token1"
    - app_id: "auth2"
      refresh_token: "token2"
    - app_id: "auth3"
      refresh_token: "token3"
`
	tmpFile := createTempConfigFile(t, content)
	defer os.Remove(tmpFile)

	cfg, err := Load(tmpFile)
	require.NoError(t, err)

	assert.Len(t, cfg.WeChat.Authorizers, 3)

	// Test GetAuthorizerByAppID
	auth, found := cfg.WeChat.GetAuthorizerByAppID("auth2")
	assert.True(t, found)
	assert.Equal(t, "auth2", auth.AppID)
	assert.Equal(t, "token2", auth.RefreshToken)

	// Test not found
	_, found = cfg.WeChat.GetAuthorizerByAppID("nonexistent")
	assert.False(t, found)
}

func TestLoad_SamePortError(t *testing.T) {
	content := `
server:
  http_port: 8080
  grpc_port: 8080
redis:
  host: localhost
  port: 6379
wechat:
  component:
    app_id: "test"
    app_secret: "test"
    verify_ticket: "test"
  authorizers:
    - app_id: "auth"
      refresh_token: "token"
`
	tmpFile := createTempConfigFile(t, content)
	defer os.Remove(tmpFile)

	cfg, err := Load(tmpFile)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "cannot be the same")
}

func TestLoad_FileNotFound(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestValidate_InvalidPortRange(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			HTTPPort: 70000, // Invalid port
			GRPCPort: 9090,
		},
		Redis: RedisConfig{
			Host: "localhost",
			Port: 6379,
		},
		WeChat: WeChatConfig{
			Component: ComponentConfig{
				AppID:        "test",
				AppSecret:    "test",
				VerifyTicket: "test",
			},
			Authorizers: []AuthorizerConfig{
				{AppID: "auth", RefreshToken: "token"},
			},
		},
	}

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPPort")
}

func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)
	return tmpFile
}
