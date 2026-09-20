package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

// SubscriptionsConfig contains only settings owned by the subscriptions
// process. Keeping it separate prevents the service from depending on the
// gateway's authentication and HTTP configuration.
type SubscriptionsConfig struct {
	DB     SubscriptionsDBConfig `yaml:"db"`
	Logger LoggerConfig          `yaml:"logger"`
	GRPC   GRPCConfig            `yaml:"grpc"`
}

type SubscriptionsDBConfig struct {
	DSN string `yaml:"dsn" env:"SUBSCRIPTIONS_DB_DSN"`
}

type GRPCConfig struct {
	Address string `yaml:"address" env:"SUBSCRIPTIONS_GRPC_ADDR" env-default:":9004"`
}

func LoadSubscriptions(path string) (*SubscriptionsConfig, error) {
	var cfg SubscriptionsConfig
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read subscriptions config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate subscriptions config: %w", err)
	}
	return &cfg, nil
}

func (c *SubscriptionsConfig) Validate() error {
	switch {
	case strings.TrimSpace(c.DB.DSN) == "":
		return errors.New("subscriptions database DSN must not be empty")
	case strings.TrimSpace(c.GRPC.Address) == "":
		return errors.New("subscriptions gRPC address must not be empty")
	default:
		return nil
	}
}
