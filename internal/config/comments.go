package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type CommentsConfig struct {
	DB      CommentsDBConfig      `yaml:"db"`
	Logger  LoggerConfig          `yaml:"logger"`
	GRPC    CommentsGRPCConfig    `yaml:"grpc"`
	Clients CommentsClientsConfig `yaml:"clients"`
}

type CommentsDBConfig struct {
	DSN string `yaml:"dsn" env:"COMMENTS_DB_DSN"`
}

type CommentsGRPCConfig struct {
	Address string `yaml:"address" env:"COMMENTS_GRPC_ADDR" env-default:":9005"`
}

type CommentsClientsConfig struct {
	PostsAddress   string `yaml:"posts_address" env:"POSTS_GRPC_ADDR"`
	ProfileAddress string `yaml:"profile_address" env:"PROFILE_GRPC_ADDR"`
}

func LoadComments(path string) (*CommentsConfig, error) {
	var cfg CommentsConfig
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read comments config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate comments config: %w", err)
	}
	return &cfg, nil
}

func (c *CommentsConfig) Validate() error {
	switch {
	case strings.TrimSpace(c.DB.DSN) == "":
		return errors.New("comments database DSN must not be empty")
	case strings.TrimSpace(c.GRPC.Address) == "":
		return errors.New("comments gRPC address must not be empty")
	case strings.TrimSpace(c.Clients.PostsAddress) == "":
		return errors.New("posts gRPC address must not be empty")
	case strings.TrimSpace(c.Clients.ProfileAddress) == "":
		return errors.New("profile gRPC address must not be empty")
	default:
		return nil
	}
}
