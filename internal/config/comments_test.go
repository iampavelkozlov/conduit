package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommentsConfigValidate(t *testing.T) {
	valid := CommentsConfig{
		DB:      CommentsDBConfig{DSN: "postgres://localhost/comments"},
		GRPC:    CommentsGRPCConfig{Address: ":9005"},
		Clients: CommentsClientsConfig{PostsAddress: "posts:9003", ProfileAddress: "profile:9002"},
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name   string
		mutate func(*CommentsConfig)
	}{
		{name: "database", mutate: func(cfg *CommentsConfig) { cfg.DB.DSN = " " }},
		{name: "server address", mutate: func(cfg *CommentsConfig) { cfg.GRPC.Address = " " }},
		{name: "posts address", mutate: func(cfg *CommentsConfig) { cfg.Clients.PostsAddress = " " }},
		{name: "profile address", mutate: func(cfg *CommentsConfig) { cfg.Clients.ProfileAddress = " " }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			require.Error(t, cfg.Validate())
		})
	}
}

func TestLoadComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "comments.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
db:
  dsn: postgres://localhost/comments
logger:
  level: info
  format: text
grpc:
  address: ":9005"
clients:
  posts_address: posts:9003
  profile_address: profile:9002
`), 0o600))
	t.Setenv("COMMENTS_DB_DSN", "postgres://localhost/override")
	t.Setenv("COMMENTS_GRPC_ADDR", ":19005")
	t.Setenv("POSTS_GRPC_ADDR", "posts:19003")
	t.Setenv("PROFILE_GRPC_ADDR", "profile:19002")
	cfg, err := LoadComments(path)
	require.NoError(t, err)
	require.Equal(t, "postgres://localhost/override", cfg.DB.DSN)
	require.Equal(t, ":19005", cfg.GRPC.Address)
	require.Equal(t, "posts:19003", cfg.Clients.PostsAddress)
	require.Equal(t, "profile:19002", cfg.Clients.ProfileAddress)

	_, err = LoadComments(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "read comments config")
}
