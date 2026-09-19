package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type FrontendConfig struct {
	API     FrontendAPIConfig     `yaml:"api"`
	HTTP    FrontendHTTPConfig    `yaml:"http"`
	Session FrontendSessionConfig `yaml:"session"`
	Logger  FrontendLoggerConfig  `yaml:"logger"`
}

type FrontendAPIConfig struct {
	URL     string        `yaml:"url" env:"FRONTEND_API_URL"`
	Timeout time.Duration `yaml:"timeout" env:"FRONTEND_API_TIMEOUT"`
}

type FrontendHTTPConfig struct {
	Addr              string        `yaml:"addr" env:"FRONTEND_ADDR"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout" env:"FRONTEND_READ_HEADER_TIMEOUT"`
	ReadTimeout       time.Duration `yaml:"read_timeout" env:"FRONTEND_READ_TIMEOUT"`
	WriteTimeout      time.Duration `yaml:"write_timeout" env:"FRONTEND_WRITE_TIMEOUT"`
	IdleTimeout       time.Duration `yaml:"idle_timeout" env:"FRONTEND_IDLE_TIMEOUT"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout" env:"FRONTEND_SHUTDOWN_TIMEOUT"`
	MaxHeaderBytes    int           `yaml:"max_header_bytes" env:"FRONTEND_MAX_HEADER_BYTES"`
	MaxFormBytes      int64         `yaml:"max_form_bytes" env:"FRONTEND_MAX_FORM_BYTES"`
}

type FrontendSessionConfig struct {
	CookieName        string        `yaml:"cookie_name" env:"FRONTEND_COOKIE_NAME"`
	RefreshCookieName string        `yaml:"refresh_cookie_name" env:"FRONTEND_REFRESH_COOKIE_NAME"`
	Secure            bool          `yaml:"secure" env:"FRONTEND_COOKIE_SECURE"`
	TTL               time.Duration `yaml:"ttl" env:"FRONTEND_COOKIE_TTL"`
}

type FrontendLoggerConfig struct {
	Level  string `yaml:"level" env:"FRONTEND_LOG_LEVEL"`
	Format string `yaml:"format" env:"FRONTEND_LOG_FORMAT"`
}

func LoadFrontend(path string) (*FrontendConfig, error) {
	var cfg FrontendConfig
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read frontend config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate frontend config: %w", err)
	}
	return &cfg, nil
}

func (c *FrontendConfig) Validate() error {
	apiURL, err := url.ParseRequestURI(c.API.URL)
	switch {
	case strings.TrimSpace(c.HTTP.Addr) == "":
		return errors.New("frontend HTTP address must not be empty")
	case err != nil || apiURL.Scheme == "" || apiURL.Host == "":
		return errors.New("frontend API URL must be absolute")
	case c.API.Timeout <= 0:
		return errors.New("frontend API timeout must be positive")
	case c.HTTP.ReadHeaderTimeout <= 0:
		return errors.New("frontend read header timeout must be positive")
	case c.HTTP.ReadTimeout <= 0:
		return errors.New("frontend read timeout must be positive")
	case c.HTTP.WriteTimeout <= 0:
		return errors.New("frontend write timeout must be positive")
	case c.HTTP.IdleTimeout <= 0:
		return errors.New("frontend idle timeout must be positive")
	case c.HTTP.ShutdownTimeout <= 0:
		return errors.New("frontend shutdown timeout must be positive")
	case c.HTTP.MaxHeaderBytes <= 0:
		return errors.New("frontend max header bytes must be positive")
	case c.HTTP.MaxFormBytes <= 0:
		return errors.New("frontend max form bytes must be positive")
	case strings.TrimSpace(c.Session.CookieName) == "":
		return errors.New("frontend cookie name must not be empty")
	case strings.TrimSpace(c.Session.RefreshCookieName) == "":
		return errors.New("frontend refresh cookie name must not be empty")
	case c.Session.RefreshCookieName == c.Session.CookieName:
		return errors.New("frontend cookie names must differ")
	case c.Session.TTL <= 0:
		return errors.New("frontend cookie TTL must be positive")
	default:
		return nil
	}
}
