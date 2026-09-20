package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProfileConfigValidate(t *testing.T) {
	valid := ProfileConfig{
		DB: ProfileDBConfig{DSN: "postgres://localhost/profile"}, GRPC: ProfileGRPCConfig{Address: ":9002"},
		Redis: ProfileRedisConfig{Address: "localhost:6379", TTL: time.Minute},
	}
	require.NoError(t, valid.Validate())
	tests := map[string]func(*ProfileConfig){
		"database": func(cfg *ProfileConfig) { cfg.DB.DSN = "" },
		"grpc":     func(cfg *ProfileConfig) { cfg.GRPC.Address = "" },
		"redis":    func(cfg *ProfileConfig) { cfg.Redis.Address = "" },
		"redis db": func(cfg *ProfileConfig) { cfg.Redis.DB = -1 },
		"ttl":      func(cfg *ProfileConfig) { cfg.Redis.TTL = 0 },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := valid
			mutate(&cfg)
			require.Error(t, cfg.Validate())
		})
	}
}

func TestLoadProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
db:
  dsn: postgres://localhost/profile
logger:
  level: info
  format: text
grpc:
  address: ":9002"
redis:
  address: localhost:6379
  db: 0
  ttl: 5m
`), 0o600))
	cfg, err := LoadProfile(path)
	require.NoError(t, err)
	require.Equal(t, 5*time.Minute, cfg.Redis.TTL)

	t.Setenv("PROFILE_GRPC_ADDR", ":19002")
	cfg, err = LoadProfile(path)
	require.NoError(t, err)
	require.Equal(t, ":19002", cfg.GRPC.Address)

	_, err = LoadProfile(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "read profile config")

	t.Setenv("PROFILE_DB_DSN", " ")
	_, err = LoadProfile(path)
	require.ErrorContains(t, err, "validate profile config")
}
