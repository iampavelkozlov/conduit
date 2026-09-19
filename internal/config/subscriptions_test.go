package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubscriptionsConfigValidate(t *testing.T) {
	valid := SubscriptionsConfig{
		DB:   SubscriptionsDBConfig{DSN: "postgres://localhost/subscriptions"},
		GRPC: GRPCConfig{Address: ":9004"},
	}
	require.NoError(t, valid.Validate())

	t.Run("empty database DSN", func(t *testing.T) {
		cfg := valid
		cfg.DB.DSN = " "
		require.Error(t, cfg.Validate())
	})
	t.Run("empty gRPC address", func(t *testing.T) {
		cfg := valid
		cfg.GRPC.Address = " "
		require.Error(t, cfg.Validate())
	})
}

func TestLoadSubscriptions(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "subscriptions.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
db:
  dsn: postgres://localhost/subscriptions
logger:
  level: info
  format: text
grpc:
  address: ":9004"
`), 0o600))

	cfg, err := LoadSubscriptions(configPath)
	require.NoError(t, err)
	require.Equal(t, "postgres://localhost/subscriptions", cfg.DB.DSN)
	require.Equal(t, ":9004", cfg.GRPC.Address)

	t.Setenv("SUBSCRIPTIONS_DB_DSN", "postgres://localhost/subscriptions_override")
	t.Setenv("SUBSCRIPTIONS_GRPC_ADDR", ":19004")
	cfg, err = LoadSubscriptions(configPath)
	require.NoError(t, err)
	require.Equal(t, "postgres://localhost/subscriptions_override", cfg.DB.DSN)
	require.Equal(t, ":19004", cfg.GRPC.Address)

	_, err = LoadSubscriptions(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "read subscriptions config")
}
