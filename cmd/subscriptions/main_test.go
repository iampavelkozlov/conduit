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
	"conduit/internal/service/follow"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestLoadConfig(t *testing.T) {
	configPath := writeConfig(t, "info", ":9004", "postgres://localhost/subscriptions")

	cfg, err := loadConfig([]string{"--config", configPath, "--addr", ":19004"})
	require.NoError(t, err)
	require.Equal(t, ":19004", cfg.GRPC.Address)

	_, err = loadConfig([]string{"--unknown"})
	require.ErrorContains(t, err, "parse flags")

	_, err = loadConfig([]string{"--config", filepath.Join(t.TempDir(), "missing.yaml")})
	require.ErrorContains(t, err, "read subscriptions config")
}

func TestRunRejectsInvalidLoggerBeforeConnecting(t *testing.T) {
	configPath := writeConfig(t, "invalid", ":9004", "postgres://localhost/subscriptions")
	err := run(t.Context(), []string{"--config", configPath})
	require.ErrorContains(t, err, "initialize logger")
}

func TestRunRejectsInvalidDatabase(t *testing.T) {
	configPath := writeConfig(t, "info", ":9004", "://")
	err := run(t.Context(), []string{"--config", configPath})
	require.ErrorContains(t, err, "initialize subscriptions database")
}

func TestRunWithStartsAndCleansUp(t *testing.T) {
	configPath := writeConfig(t, "info", "127.0.0.1:0", "postgres://localhost/subscriptions")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cleaned := false
	initialized := false
	err := runWith(ctx, []string{"--config", configPath}, func(_ context.Context, _ *config.SubscriptionsConfig, appLogger *slog.Logger) (*follow.Service, func(), error) {
		initialized = true
		return follow.New(nil, appLogger), func() { cleaned = true }, nil
	}, net.Listen)
	require.NoError(t, err)
	require.True(t, initialized)
	require.True(t, cleaned)
}

func TestRunWithReportsInitializationAndListenErrors(t *testing.T) {
	configPath := writeConfig(t, "info", "127.0.0.1:0", "postgres://localhost/subscriptions")
	wantErr := errors.New("unavailable")

	err := runWith(t.Context(), []string{"--config", configPath}, func(context.Context, *config.SubscriptionsConfig, *slog.Logger) (*follow.Service, func(), error) {
		return nil, nil, wantErr
	}, net.Listen)
	require.ErrorIs(t, err, wantErr)

	cleaned := false
	err = runWith(t.Context(), []string{"--config", configPath}, func(_ context.Context, _ *config.SubscriptionsConfig, appLogger *slog.Logger) (*follow.Service, func(), error) {
		return follow.New(nil, appLogger), func() { cleaned = true }, nil
	}, func(string, string) (net.Listener, error) {
		return nil, wantErr
	})
	require.ErrorIs(t, err, wantErr)
	require.True(t, cleaned)
}

func TestServePublishesHealthAndShutsDown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	server, healthServer := newGRPCServer(follow.New(nil, slog.New(slog.DiscardHandler)))
	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() {
		errCh <- serve(ctx, listener, server, healthServer, slog.New(slog.DiscardHandler))
	}()

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
		t.Fatal("subscriptions server did not stop")
	}
}

func writeConfig(t *testing.T, level, address, dsn string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "subscriptions.yaml")
	contents := []byte("db:\n  dsn: \"" + dsn + "\"\nlogger:\n  level: \"" + level + "\"\n  format: text\ngrpc:\n  address: \"" + address + "\"\n")
	require.NoError(t, os.WriteFile(path, contents, 0o600))
	return path
}
