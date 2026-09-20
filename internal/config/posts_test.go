package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostsConfigValidate(t *testing.T) {
	valid := PostsConfig{
		DB: PostsDBConfig{DSN: "postgres://localhost/posts"}, GRPC: PostsGRPCConfig{Address: ":9003"},
		Profile: ProfileServiceTarget{Target: "localhost:9002"}, Subscriptions: SubscriptionsServiceTarget{Target: "localhost:9004"},
	}
	require.NoError(t, valid.Validate())

	tests := map[string]func(*PostsConfig){
		"database":       func(cfg *PostsConfig) { cfg.DB.DSN = " " },
		"listen address": func(cfg *PostsConfig) { cfg.GRPC.Address = " " },
		"profile":        func(cfg *PostsConfig) { cfg.Profile.Target = " " },
		"subscriptions":  func(cfg *PostsConfig) { cfg.Subscriptions.Target = " " },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := valid
			mutate(&cfg)
			require.Error(t, cfg.Validate())
		})
	}
}

func TestLoadPosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "posts.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
db:
  dsn: postgres://localhost/posts
logger:
  level: info
  format: text
grpc:
  address: ":9003"
profile:
  target: localhost:9002
subscriptions:
  target: localhost:9004
`), 0o600))
	cfg, err := LoadPosts(path)
	require.NoError(t, err)
	require.Equal(t, "postgres://localhost/posts", cfg.DB.DSN)
	require.Equal(t, "localhost:9002", cfg.Profile.Target)

	t.Setenv("POSTS_GRPC_ADDR", ":19003")
	t.Setenv("PROFILE_GRPC_ADDR", "profile:9002")
	t.Setenv("SUBSCRIPTIONS_GRPC_ADDR", "subscriptions:9004")
	cfg, err = LoadPosts(path)
	require.NoError(t, err)
	require.Equal(t, ":19003", cfg.GRPC.Address)
	require.Equal(t, "profile:9002", cfg.Profile.Target)
	require.Equal(t, "subscriptions:9004", cfg.Subscriptions.Target)

	_, err = LoadPosts(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "read posts config")

	t.Setenv("POSTS_DB_DSN", " ")
	_, err = LoadPosts(path)
	require.ErrorContains(t, err, "validate posts config")
}
