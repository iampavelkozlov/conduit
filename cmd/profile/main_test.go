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
	profilev1 "conduit/internal/gen/grpc/conduit/profile/v1"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type stubProfileServer struct {
	profilev1.UnimplementedProfileServiceServer
}

func TestLoadConfig(t *testing.T) {
	path := writeConfig(t, "info", "127.0.0.1:9001")
	cfg, err := loadConfig([]string{"--config", path, "--addr", "127.0.0.1:19001"})
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:19001", cfg.GRPC.Address)

	_, err = loadConfig([]string{"--unknown"})
	require.ErrorContains(t, err, "parse flags")
	_, err = loadConfig([]string{"--config", filepath.Join(t.TempDir(), "missing.yaml")})
	require.Error(t, err)
}

func TestRunRejectsInvalidLoggerBeforeDatabase(t *testing.T) {
	t.Setenv("LOG_LEVEL", "invalid")
	t.Setenv("LOG_FORMAT", "text")
	err := run(t.Context(), []string{"--config", writeConfig(t, "invalid", "127.0.0.1:9001")})
	require.ErrorContains(t, err, "initialize logger")
}

func TestMainRunReportsArgumentError(t *testing.T) {
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	os.Args = []string{"profile", "--unknown"}
	require.ErrorContains(t, mainRun(), "parse flags")
}

func TestServePublishesHealthAndShutsDown(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	server, healthServer := newGRPCServer(&stubProfileServer{})
	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() { errCh <- serve(ctx, listener, server, healthServer, slog.New(slog.DiscardHandler)) }()

	connection, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	checkCtx, checkCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer checkCancel()
	response, err := grpc_health_v1.NewHealthClient(connection).Check(checkCtx, &grpc_health_v1.HealthCheckRequest{Service: serviceName})
	require.NoError(t, err)
	require.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, response.GetStatus())

	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("profile server did not stop")
	}
}

func TestRunWithLifecycleAndErrors(t *testing.T) {
	path := writeConfig(t, "info", "127.0.0.1:0")
	initializer := func(context.Context, *config.ProfileConfig, *slog.Logger) (*grpc.Server, *health.Server, func(), error) {
		server, healthServer := newGRPCServer(&stubProfileServer{})
		return server, healthServer, func() {}, nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.NoError(t, runWith(ctx, []string{"--config", path}, initializer, new(net.ListenConfig).Listen))

	wantErr := errors.New("unavailable")
	err := runWith(t.Context(), []string{"--config", path}, func(context.Context, *config.ProfileConfig, *slog.Logger) (*grpc.Server, *health.Server, func(), error) {
		return nil, nil, nil, wantErr
	}, new(net.ListenConfig).Listen)
	require.ErrorIs(t, err, wantErr)

	cleaned := false
	err = runWith(t.Context(), []string{"--config", path}, func(context.Context, *config.ProfileConfig, *slog.Logger) (*grpc.Server, *health.Server, func(), error) {
		server, healthServer := newGRPCServer(&stubProfileServer{})
		return server, healthServer, func() { cleaned = true }, nil
	}, func(context.Context, string, string) (net.Listener, error) { return nil, wantErr })
	require.ErrorIs(t, err, wantErr)
	require.True(t, cleaned)
}

func TestServeReportsListenerFailure(t *testing.T) {
	wantErr := errors.New("accept failed")
	server, healthServer := newGRPCServer(&stubProfileServer{})
	err := serve(t.Context(), profileErrorListener{err: wantErr}, server, healthServer, slog.New(slog.DiscardHandler))
	require.ErrorContains(t, err, "serve gRPC")
	require.ErrorIs(t, err, wantErr)
}

type profileErrorListener struct{ err error }

func (l profileErrorListener) Accept() (net.Conn, error) { return nil, l.err }
func (profileErrorListener) Close() error                { return nil }
func (profileErrorListener) Addr() net.Addr              { return profileTestAddr("error-listener") }

type profileTestAddr string

func (a profileTestAddr) Network() string { return string(a) }
func (a profileTestAddr) String() string  { return string(a) }

func writeConfig(t *testing.T, level, address string) string {
	t.Helper()
	t.Setenv("LOG_LEVEL", level)
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("PROFILE_DB_DSN", "postgres://localhost/profile")
	t.Setenv("PROFILE_GRPC_ADDR", address)
	t.Setenv("PROFILE_REDIS_ADDR", "localhost:6379")
	path := filepath.Join(t.TempDir(), "profile.yaml")
	contents := []byte("db:\n  dsn: postgres://localhost/profile\nlogger:\n  level: " + level + "\n  format: text\ngrpc:\n  address: \"" + address + "\"\nredis:\n  address: localhost:6379\n  db: 0\n  ttl: 10m\n")
	require.NoError(t, os.WriteFile(path, contents, 0o600))
	return path
}
