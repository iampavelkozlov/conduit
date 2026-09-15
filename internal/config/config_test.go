package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{
		DB: DBConfig{DSN: "postgres://localhost/conduit"},
		Auth: AuthConfig{
			JWTSecret:       "a-secret-with-at-least-32-bytes-long",
			PasswordPepper:  "a-long-password-pepper",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: time.Hour,
		},
		HTTP: HTTPConfig{AllowedOrigins: []string{"https://example.com"}},
	}
	require.NoError(t, valid.Validate())

	tests := map[string]func(*Config){
		"empty database DSN":    func(c *Config) { c.DB.DSN = "" },
		"short JWT secret":      func(c *Config) { c.Auth.JWTSecret = "short" },
		"short password pepper": func(c *Config) { c.Auth.PasswordPepper = "short" },
		"invalid access TTL":    func(c *Config) { c.Auth.AccessTokenTTL = 0 },
		"invalid refresh TTL":   func(c *Config) { c.Auth.RefreshTokenTTL = c.Auth.AccessTokenTTL },
		"empty allowed origins": func(c *Config) { c.HTTP.AllowedOrigins = nil },
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
	t.Setenv("DB_DSN", "postgres://localhost/conduit")
	t.Setenv("AUTH_JWT_SECRET", "a-secret-with-at-least-32-bytes-long")
	t.Setenv("AUTH_PASSWORD_PEPPER", "a-long-password-pepper")
	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "15m")
	t.Setenv("AUTH_REFRESH_TOKEN_TTL", "1h")
	t.Setenv("HTTP_ALLOWED_ORIGINS", "https://example.com")

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
db:
  dsn: postgres://localhost/conduit
logger:
  level: info
  format: text
auth:
  jwt_secret: a-secret-with-at-least-32-bytes-long
  password_pepper: a-long-password-pepper
  access_token_ttl: 15m
  refresh_token_ttl: 1h
http:
  allowed_origins:
    - https://example.com
`), 0o600))

	cfg, err := Load(configPath)
	require.NoError(t, err)
	require.Equal(t, "postgres://localhost/conduit", cfg.DB.DSN)
	require.Equal(t, []string{"https://example.com"}, cfg.HTTP.AllowedOrigins)

	_, err = Load(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "read config")

	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "invalid")
	_, err = Load(configPath)
	require.ErrorContains(t, err, "read config")

	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "15m")
	t.Setenv("AUTH_JWT_SECRET", "short")
	_, err = Load(configPath)
	require.ErrorContains(t, err, "validate config")
}
