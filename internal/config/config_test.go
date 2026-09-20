package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func validGatewayConfig() Config {
	return Config{
		HTTP: HTTPConfig{AllowedOrigins: []string{"https://example.com"}},
		Services: RemoteServicesConfig{
			Timeout:       time.Second,
			Auth:          AuthGRPCClientConfig{Target: "auth:9001"},
			Profile:       ProfileGRPCClientConfig{Target: "profile:9002"},
			Posts:         PostsGRPCClientConfig{Target: "posts:9003"},
			Comments:      CommentsGRPCClientConfig{Target: "comments:9005"},
			Subscriptions: SubscriptionsGRPCClientConfig{Target: "subscriptions:9004"},
		},
	}
}

func TestConfigValidate(t *testing.T) {
	valid := validGatewayConfig()
	require.NoError(t, valid.Validate())

	tests := map[string]func(*Config){
		"empty allowed origins": func(c *Config) { c.HTTP.AllowedOrigins = nil },
		"invalid RPC timeout":   func(c *Config) { c.Services.Timeout = 0 },
		"missing auth target":   func(c *Config) { c.Services.Auth.Target = "" },
		"blank profile target":  func(c *Config) { c.Services.Profile.Target = " " },
		"missing posts target":  func(c *Config) { c.Services.Posts.Target = "" },
		"missing comments target": func(c *Config) {
			c.Services.Comments.Target = ""
		},
		"missing subscriptions target": func(c *Config) {
			c.Services.Subscriptions.Target = ""
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := valid
			mutate(&cfg)
			require.Error(t, cfg.Validate())
		})
	}
}

func TestLoad(t *testing.T) {
	t.Setenv("SERVICES_GRPC_TIMEOUT", "2s")
	t.Setenv("AUTH_GRPC_TARGET", "auth:9001")
	t.Setenv("PROFILE_GRPC_TARGET", "profile:9002")
	t.Setenv("POSTS_GRPC_TARGET", "posts:9003")
	t.Setenv("COMMENTS_GRPC_TARGET", "comments:9005")
	t.Setenv("SUBSCRIPTIONS_GRPC_TARGET", "subscriptions:9004")
	t.Setenv("HTTP_ALLOWED_ORIGINS", "https://example.com")

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
logger:
  level: info
  format: text
http:
  allowed_origins:
    - https://example.com
services:
  timeout: 3s
  auth: {target: localhost:9001}
  profile: {target: localhost:9002}
  posts: {target: localhost:9003}
  comments: {target: localhost:9005}
  subscriptions: {target: localhost:9004}
`), 0o600))

	cfg, err := Load(configPath)
	require.NoError(t, err)
	require.Equal(t, 2*time.Second, cfg.Services.Timeout)
	require.Equal(t, "auth:9001", cfg.Services.Auth.Target)
	require.Equal(t, []string{"https://example.com"}, cfg.HTTP.AllowedOrigins)

	_, err = Load(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "read config")

	t.Setenv("SERVICES_GRPC_TIMEOUT", "invalid")
	_, err = Load(configPath)
	require.ErrorContains(t, err, "read config")

	t.Setenv("SERVICES_GRPC_TIMEOUT", "2s")
	t.Setenv("AUTH_GRPC_TARGET", "")
	_, err = Load(configPath)
	require.ErrorContains(t, err, "validate config")
}
