package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type OutboxRelayConfig struct {
	DB     OutboxDBConfig     `yaml:"db"`
	Logger LoggerConfig       `yaml:"logger"`
	Kafka  OutboxKafkaConfig  `yaml:"kafka"`
	Relay  OutboxWorkerConfig `yaml:"relay"`
}

type OutboxDBConfig struct {
	DSN string `yaml:"dsn" env:"OUTBOX_DB_DSN"`
}

type OutboxKafkaConfig struct {
	Brokers  []string `yaml:"brokers" env:"KAFKA_BROKERS" env-separator:","`
	ClientID string   `yaml:"client_id" env:"KAFKA_CLIENT_ID"`
	Username string   `yaml:"username" env:"KAFKA_USERNAME"`
	Password string   `yaml:"password" env:"KAFKA_PASSWORD"`
	TLS      bool     `yaml:"tls" env:"KAFKA_TLS"`
}

type OutboxWorkerConfig struct {
	Source         string        `yaml:"source" env:"OUTBOX_SOURCE"`
	BatchSize      int           `yaml:"batch_size" env:"OUTBOX_BATCH_SIZE"`
	PollInterval   time.Duration `yaml:"poll_interval" env:"OUTBOX_POLL_INTERVAL"`
	LockTimeout    time.Duration `yaml:"lock_timeout" env:"OUTBOX_LOCK_TIMEOUT"`
	RetryDelay     time.Duration `yaml:"retry_delay" env:"OUTBOX_RETRY_DELAY"`
	MaxRetryDelay  time.Duration `yaml:"max_retry_delay" env:"OUTBOX_MAX_RETRY_DELAY"`
	MetricsAddress string        `yaml:"metrics_address" env:"OUTBOX_METRICS_ADDR"`
}

func LoadOutboxRelay(path string) (*OutboxRelayConfig, error) {
	var cfg OutboxRelayConfig
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read outbox relay config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate outbox relay config: %w", err)
	}
	return &cfg, nil
}

func (c *OutboxRelayConfig) Validate() error {
	switch {
	case strings.TrimSpace(c.DB.DSN) == "":
		return errors.New("outbox database DSN must not be empty")
	case len(c.Kafka.Brokers) == 0:
		return errors.New("at least one Kafka broker is required")
	case strings.TrimSpace(c.Kafka.ClientID) == "":
		return errors.New("kafka client ID must not be empty")
	case strings.TrimSpace(c.Relay.Source) == "":
		return errors.New("outbox source must not be empty")
	case c.Relay.BatchSize <= 0:
		return errors.New("outbox batch size must be positive")
	case c.Relay.PollInterval <= 0 || c.Relay.LockTimeout <= 0 || c.Relay.RetryDelay <= 0:
		return errors.New("outbox durations must be positive")
	case c.Relay.MaxRetryDelay < c.Relay.RetryDelay:
		return errors.New("outbox max retry delay must be at least retry delay")
	case strings.TrimSpace(c.Relay.MetricsAddress) == "":
		return errors.New("outbox metrics address must not be empty")
	default:
		return nil
	}
}
