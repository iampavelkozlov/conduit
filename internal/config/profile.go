package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type ProfileConfig struct {
	DB     ProfileDBConfig    `yaml:"db"`
	Logger LoggerConfig       `yaml:"logger"`
	GRPC   ProfileGRPCConfig  `yaml:"grpc"`
	Redis  ProfileRedisConfig `yaml:"redis"`
}

type ProfileDBConfig struct {
	DSN string `yaml:"dsn" env:"PROFILE_DB_DSN"`
}

type ProfileGRPCConfig struct {
	Address string `yaml:"address" env:"PROFILE_GRPC_ADDR" env-default:":9002"`
}

type ProfileRedisConfig struct {
	Address  string        `yaml:"address" env:"PROFILE_REDIS_ADDR"`
	Password string        `yaml:"password" env:"PROFILE_REDIS_PASSWORD"`
	DB       int           `yaml:"db" env:"PROFILE_REDIS_DB"`
	TTL      time.Duration `yaml:"ttl" env:"PROFILE_CACHE_TTL" env-default:"10m"`
}

func LoadProfile(path string) (*ProfileConfig, error) {
	var cfg ProfileConfig
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read profile config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate profile config: %w", err)
	}
	return &cfg, nil
}

func (c *ProfileConfig) Validate() error {
	switch {
	case strings.TrimSpace(c.DB.DSN) == "":
		return errors.New("profile database DSN must not be empty")
	case strings.TrimSpace(c.GRPC.Address) == "":
		return errors.New("profile gRPC address must not be empty")
	case strings.TrimSpace(c.Redis.Address) == "":
		return errors.New("profile Redis address must not be empty")
	case c.Redis.DB < 0:
		return errors.New("profile Redis database must not be negative")
	case c.Redis.TTL <= 0:
		return errors.New("profile cache TTL must be positive")
	default:
		return nil
	}
}
