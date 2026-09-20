package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadAuthService(t *testing.T) {
	t.Setenv("AUTH_JWT_SECRET", "a-secret-with-at-least-32-bytes-long")
	t.Setenv("AUTH_PASSWORD_PEPPER", "a-long-password-pepper")
	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "15m")
	t.Setenv("AUTH_REFRESH_TOKEN_TTL", "1h")
	path := filepath.Join(t.TempDir(), "auth.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
db:
  dsn: postgres://localhost/auth
logger:
  level: info
  format: text
auth:
  jwt_secret: a-secret-with-at-least-32-bytes-long
  password_pepper: a-long-password-pepper
  access_token_ttl: 15m
  refresh_token_ttl: 1h
grpc:
  address: ":9001"
`), 0o600))
	cfg, err := LoadAuthService(path)
	require.NoError(t, err)
	require.Equal(t, ":9001", cfg.GRPC.Address)

	t.Setenv("AUTH_DB_DSN", "postgres://localhost/auth_override")
	t.Setenv("AUTH_GRPC_ADDR", ":19001")
	cfg, err = LoadAuthService(path)
	require.NoError(t, err)
	require.Equal(t, "postgres://localhost/auth_override", cfg.DB.DSN)
	require.Equal(t, ":19001", cfg.GRPC.Address)
}

func TestLoadAuthServiceReportsReadAndValidationErrors(t *testing.T) {
	_, err := LoadAuthService(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "read auth config")

	path := filepath.Join(t.TempDir(), "auth.yaml")
	require.NoError(t, os.WriteFile(path, []byte("db:\n  dsn: ''\n"), 0o600))
	_, err = LoadAuthService(path)
	require.ErrorContains(t, err, "validate auth config")
}

func TestAuthServiceConfigValidate(t *testing.T) {
	valid := AuthServiceConfig{DB: AuthDBConfig{DSN: "db"}, GRPC: AuthGRPCConfig{Address: ":1"}, Auth: AuthConfig{
		JWTSecret: "a-secret-with-at-least-32-bytes-long", PasswordPepper: "a-long-password-pepper", AccessTokenTTL: 1, RefreshTokenTTL: 2,
	}}
	require.NoError(t, valid.Validate())
	tests := []func(*AuthServiceConfig){
		func(c *AuthServiceConfig) { c.DB.DSN = "" },
		func(c *AuthServiceConfig) { c.GRPC.Address = "" },
		func(c *AuthServiceConfig) { c.Auth.JWTSecret = "short" },
		func(c *AuthServiceConfig) { c.Auth.PasswordPepper = "short" },
		func(c *AuthServiceConfig) { c.Auth.AccessTokenTTL = 0 },
		func(c *AuthServiceConfig) { c.Auth.RefreshTokenTTL = c.Auth.AccessTokenTTL },
	}
	for i, mutate := range tests {
		t.Run(string(rune('a'+i)), func(t *testing.T) { cfg := valid; mutate(&cfg); require.Error(t, cfg.Validate()) })
	}
}
