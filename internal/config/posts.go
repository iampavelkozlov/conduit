package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

// PostsConfig contains settings owned by the posts process.
type PostsConfig struct {
	DB            PostsDBConfig              `yaml:"db"`
	Logger        LoggerConfig               `yaml:"logger"`
	GRPC          PostsGRPCConfig            `yaml:"grpc"`
	Profile       ProfileServiceTarget       `yaml:"profile"`
	Subscriptions SubscriptionsServiceTarget `yaml:"subscriptions"`
}

type PostsDBConfig struct {
	DSN string `yaml:"dsn" env:"POSTS_DB_DSN"`
}

type PostsGRPCConfig struct {
	Address string `yaml:"address" env:"POSTS_GRPC_ADDR" env-default:":9003"`
}

type ProfileServiceTarget struct {
	Target string `yaml:"target" env:"PROFILE_GRPC_ADDR"`
}

type SubscriptionsServiceTarget struct {
	Target string `yaml:"target" env:"SUBSCRIPTIONS_GRPC_ADDR"`
}

func LoadPosts(path string) (*PostsConfig, error) {
	var cfg PostsConfig
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read posts config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate posts config: %w", err)
	}
	return &cfg, nil
}

func (c *PostsConfig) Validate() error {
	switch {
	case strings.TrimSpace(c.DB.DSN) == "":
		return errors.New("posts database DSN must not be empty")
	case strings.TrimSpace(c.GRPC.Address) == "":
		return errors.New("posts gRPC address must not be empty")
	case strings.TrimSpace(c.Profile.Target) == "":
		return errors.New("profile gRPC target must not be empty")
	case strings.TrimSpace(c.Subscriptions.Target) == "":
		return errors.New("subscriptions gRPC target must not be empty")
	default:
		return nil
	}
}
