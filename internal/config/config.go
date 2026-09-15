package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	DB     DBConfig     `yaml:"db"`
	Logger LoggerConfig `yaml:"logger"`
	Auth   AuthConfig   `yaml:"auth"`
	HTTP   HTTPConfig   `yaml:"http"`
}

type DBConfig struct {
	DSN string `yaml:"dsn" env:"DB_DSN"`
}

type LoggerConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"  env-default:"info"`
	Format string `yaml:"format" env:"LOG_FORMAT" env-default:"text"` // text | json
}

type AuthConfig struct {
	JWTSecret       string        `yaml:"jwt_secret" env:"AUTH_JWT_SECRET"`
	PasswordPepper  string        `yaml:"password_pepper" env:"AUTH_PASSWORD_PEPPER"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl" env:"AUTH_ACCESS_TOKEN_TTL" env-default:"15m"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl" env:"AUTH_REFRESH_TOKEN_TTL" env-default:"720h"`
}

type HTTPConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins" env:"HTTP_ALLOWED_ORIGINS" env-separator:"," env-default:"*"`
}

func Load(path string) (*Config, error) {
	var cfg Config

	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	switch {
	case strings.TrimSpace(c.DB.DSN) == "":
		return errors.New("database DSN must not be empty")
	case len(c.Auth.JWTSecret) < 32:
		return errors.New("JWT secret must contain at least 32 bytes")
	case len(c.Auth.PasswordPepper) < 16:
		return errors.New("password pepper must contain at least 16 bytes")
	case c.Auth.AccessTokenTTL <= 0:
		return errors.New("access token TTL must be positive")
	case c.Auth.RefreshTokenTTL <= c.Auth.AccessTokenTTL:
		return errors.New("refresh token TTL must exceed access token TTL")
	case len(c.HTTP.AllowedOrigins) == 0:
		return errors.New("at least one allowed HTTP origin is required")
	default:
		return nil
	}
}
