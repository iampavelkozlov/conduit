package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadOutboxRelay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
db:
  dsn: postgres://localhost/outbox
logger:
  level: info
  format: text
kafka:
  brokers: [localhost:9092]
  client_id: relay
relay:
  source: subscriptions
  batch_size: 10
  poll_interval: 100ms
  lock_timeout: 30s
  retry_delay: 1s
  max_retry_delay: 1m
  metrics_address: ":9100"
`), 0o600))
	t.Setenv("OUTBOX_DB_DSN", "postgres://localhost/override")
	t.Setenv("KAFKA_BROKERS", "one:9092,two:9092")
	t.Setenv("OUTBOX_SOURCE", "posts")
	cfg, err := LoadOutboxRelay(path)
	require.NoError(t, err)
	require.Equal(t, "postgres://localhost/override", cfg.DB.DSN)
	require.Equal(t, []string{"one:9092", "two:9092"}, cfg.Kafka.Brokers)
	require.Equal(t, "posts", cfg.Relay.Source)
	require.Equal(t, 100*time.Millisecond, cfg.Relay.PollInterval)

	_, err = LoadOutboxRelay(filepath.Join(t.TempDir(), "missing"))
	require.ErrorContains(t, err, "read outbox relay config")

	t.Setenv("OUTBOX_SOURCE", " ")
	_, err = LoadOutboxRelay(path)
	require.ErrorContains(t, err, "validate outbox relay config")
}

func TestOutboxRelayConfigValidation(t *testing.T) {
	valid := OutboxRelayConfig{
		DB: OutboxDBConfig{DSN: "dsn"}, Kafka: OutboxKafkaConfig{Brokers: []string{"broker"}, ClientID: "relay"},
		Relay: OutboxWorkerConfig{Source: "posts", BatchSize: 1, PollInterval: time.Second, LockTimeout: time.Second, RetryDelay: time.Second, MaxRetryDelay: time.Minute, MetricsAddress: ":9100"},
	}
	require.NoError(t, valid.Validate())
	tests := []func(*OutboxRelayConfig){
		func(cfg *OutboxRelayConfig) { cfg.DB.DSN = "" },
		func(cfg *OutboxRelayConfig) { cfg.Kafka.Brokers = nil },
		func(cfg *OutboxRelayConfig) { cfg.Kafka.ClientID = "" },
		func(cfg *OutboxRelayConfig) { cfg.Relay.Source = "" },
		func(cfg *OutboxRelayConfig) { cfg.Relay.BatchSize = 0 },
		func(cfg *OutboxRelayConfig) { cfg.Relay.PollInterval = 0 },
		func(cfg *OutboxRelayConfig) { cfg.Relay.MaxRetryDelay = 0 },
		func(cfg *OutboxRelayConfig) { cfg.Relay.MetricsAddress = "" },
	}
	for i, mutate := range tests {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			cfg := valid
			mutate(&cfg)
			require.Error(t, cfg.Validate())
		})
	}
}
