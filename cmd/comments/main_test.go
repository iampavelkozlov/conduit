package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"conduit/internal/config"
	"conduit/internal/service/comment"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestLoadConfig(t *testing.T) {
	path := writeConfig(t, "info", ":9005", "postgres://localhost/comments")
	cfg, err := loadConfig([]string{"--config", path, "--addr", ":19005"})
	require.NoError(t, err)
	require.Equal(t, ":19005", cfg.GRPC.Address)

	_, err = loadConfig([]string{"--unknown"})
	require.ErrorContains(t, err, "parse flags")
	_, err = loadConfig([]string{"--config", filepath.Join(t.TempDir(), "missing.yaml")})
	require.ErrorContains(t, err, "read comments config")
}

func TestRunRejectsInvalidLoggerAndDatabase(t *testing.T) {
	t.Setenv("LOG_LEVEL", "invalid")
	t.Setenv("COMMENTS_DB_DSN", "postgres://localhost/comments")
	err := run(t.Context(), []string{"--config", writeConfig(t, "invalid", ":9005", "postgres://localhost/comments")})
	require.ErrorContains(t, err, "initialize logger")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("COMMENTS_DB_DSN", "://")
	err = run(t.Context(), []string{"--config", writeConfig(t, "info", ":9005", "://")})
	require.ErrorContains(t, err, "initialize comments service")
}

func TestMainRunReportsArgumentError(t *testing.T) {
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	os.Args = []string{"comments", "--unknown"}
	require.ErrorContains(t, mainRun(), "parse flags")
}

func TestRunWithLifecycleAndErrors(t *testing.T) {
	path := writeConfig(t, "info", "127.0.0.1:0", "postgres://localhost/comments")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cleaned := false
	err := runWith(ctx, []string{"--config", path}, func(context.Context, *config.CommentsConfig, *slog.Logger) (*comment.Service, func(), error) {
		return comment.New(nil, nil, nil), func() { cleaned = true }, nil
	}, net.Listen)
	require.NoError(t, err)
	require.True(t, cleaned)

	wantErr := errors.New("unavailable")
	err = runWith(t.Context(), []string{"--config", path}, func(context.Context, *config.CommentsConfig, *slog.Logger) (*comment.Service, func(), error) {
		return nil, nil, wantErr
	}, net.Listen)
	require.ErrorIs(t, err, wantErr)

	cleaned = false
	err = runWith(t.Context(), []string{"--config", path}, func(context.Context, *config.CommentsConfig, *slog.Logger) (*comment.Service, func(), error) {
		return comment.New(nil, nil, nil), func() { cleaned = true }, nil
	}, func(string, string) (net.Listener, error) { return nil, wantErr })
	require.ErrorIs(t, err, wantErr)
	require.True(t, cleaned)
}

func TestServePublishesHealthAndShutsDown(t *testing.T) {
	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	server, healthServer := newGRPCServer(comment.New(nil, nil, nil))
	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() { errCh <- serve(ctx, listener, server, healthServer, slog.New(slog.DiscardHandler)) }()

	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	checkCtx, checkCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer checkCancel()
	response, err := grpc_health_v1.NewHealthClient(conn).Check(checkCtx, &grpc_health_v1.HealthCheckRequest{Service: serviceName})
	require.NoError(t, err)
	require.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, response.GetStatus())

	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("comments server did not stop")
	}
}

func TestServeReportsListenerFailure(t *testing.T) {
	wantErr := errors.New("accept failed")
	server, healthServer := newGRPCServer(comment.New(nil, nil, nil))
	err := serve(t.Context(), errorListener{err: wantErr}, server, healthServer, slog.New(slog.DiscardHandler))
	require.ErrorContains(t, err, "serve gRPC")
	require.ErrorIs(t, err, wantErr)
}

type errorListener struct{ err error }

func (l errorListener) Accept() (net.Conn, error) { return nil, l.err }
func (errorListener) Close() error                { return nil }
func (errorListener) Addr() net.Addr              { return testAddr("error-listener") }

type testAddr string

func (a testAddr) Network() string { return string(a) }
func (a testAddr) String() string  { return string(a) }

func writeConfig(t *testing.T, level, address, dsn string) string {
	t.Helper()
	t.Setenv("LOG_LEVEL", level)
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("COMMENTS_DB_DSN", dsn)
	t.Setenv("COMMENTS_GRPC_ADDR", address)
	path := filepath.Join(t.TempDir(), "comments.yaml")
	contents := []byte("db:\n  dsn: \"" + dsn + "\"\nlogger:\n  level: \"" + level + "\"\n  format: text\ngrpc:\n  address: \"" + address + "\"\nclients:\n  posts_address: posts:9003\n  profile_address: profile:9002\n")
	require.NoError(t, os.WriteFile(path, contents, 0o600))
	return path
}
