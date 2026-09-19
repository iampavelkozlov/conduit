package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type AuthServiceConfig struct {
	DB     AuthDBConfig   `yaml:"db"`
	Logger LoggerConfig   `yaml:"logger"`
	Auth   AuthConfig     `yaml:"auth"`
	GRPC   AuthGRPCConfig `yaml:"grpc"`
}

type AuthDBConfig struct {
	DSN string `yaml:"dsn" env:"AUTH_DB_DSN"`
}

type AuthGRPCConfig struct {
	Address string `yaml:"address" env:"AUTH_GRPC_ADDR" env-default:":9001"`
}

func LoadAuthService(path string) (*AuthServiceConfig, error) {
	var cfg AuthServiceConfig
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read auth config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate auth config: %w", err)
	}
	return &cfg, nil
}

func (c *AuthServiceConfig) Validate() error {
	switch {
	case strings.TrimSpace(c.DB.DSN) == "":
		return errors.New("auth database DSN must not be empty")
	case strings.TrimSpace(c.GRPC.Address) == "":
		return errors.New("auth gRPC address must not be empty")
	case len(c.Auth.JWTSecret) < 32:
		return errors.New("JWT secret must contain at least 32 bytes")
	case len(c.Auth.PasswordPepper) < 16:
		return errors.New("password pepper must contain at least 16 bytes")
	case c.Auth.AccessTokenTTL <= 0:
		return errors.New("access token TTL must be positive")
	case c.Auth.RefreshTokenTTL <= c.Auth.AccessTokenTTL:
		return errors.New("refresh token TTL must exceed access token TTL")
	default:
		return nil
	}
}
