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
	authservice "conduit/internal/service/auth"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestLoadConfig(t *testing.T) {
	path := writeAuthConfig(t, "info", "postgres://localhost/auth")
	cfg, err := loadConfig([]string{"--config", path, "--addr", ":19001"})
	require.NoError(t, err)
	require.Equal(t, ":19001", cfg.GRPC.Address)
	_, err = loadConfig([]string{"--unknown"})
	require.ErrorContains(t, err, "parse flags")
	_, err = loadConfig([]string{"--config", filepath.Join(t.TempDir(), "missing.yaml")})
	require.Error(t, err)
}

func TestRunRejectsConfigurationBeforeServing(t *testing.T) {
	path := writeAuthConfig(t, "invalid", "postgres://localhost/auth")
	require.ErrorContains(t, run(t.Context(), []string{"--config", path}), "initialize logger")
	path = writeAuthConfig(t, "info", "://")
	require.ErrorContains(t, run(t.Context(), []string{"--config", path}), "initialize auth database")
}

func TestMainRunReportsArgumentError(t *testing.T) {
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	os.Args = []string{"auth", "--unknown"}
	require.ErrorContains(t, mainRun(), "parse flags")
}

func TestRunWithStartsAndCleansUp(t *testing.T) {
	path := writeAuthConfig(t, "info", "postgres://localhost/auth")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cleaned := false
	err := runWith(ctx, []string{"--config", path}, func(_ context.Context, cfg *config.AuthServiceConfig, logger *slog.Logger) (*authservice.Service, func(), error) {
		return authservice.New(nil, nil, cfg.Auth, logger), func() { cleaned = true }, nil
	}, net.Listen)
	require.NoError(t, err)
	require.True(t, cleaned)
}

func TestRunWithReportsInitializationAndListenErrors(t *testing.T) {
	path := writeAuthConfig(t, "info", "postgres://localhost/auth")
	wantErr := errors.New("unavailable")
	err := runWith(t.Context(), []string{"--config", path}, func(context.Context, *config.AuthServiceConfig, *slog.Logger) (*authservice.Service, func(), error) {
		return nil, nil, wantErr
	}, net.Listen)
	require.ErrorIs(t, err, wantErr)

	cleaned := false
	err = runWith(t.Context(), []string{"--config", path}, func(_ context.Context, cfg *config.AuthServiceConfig, logger *slog.Logger) (*authservice.Service, func(), error) {
		return authservice.New(nil, nil, cfg.Auth, logger), func() { cleaned = true }, nil
	}, func(string, string) (net.Listener, error) { return nil, wantErr })
	require.ErrorIs(t, err, wantErr)
	require.True(t, cleaned)
}

func TestRunWithReportsConfigurationError(t *testing.T) {
	err := runWith(t.Context(), []string{"--config", filepath.Join(t.TempDir(), "missing.yaml")}, nil, nil)
	require.Error(t, err)
}

func TestServeHealthAndShutdown(t *testing.T) {
	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	cfg := config.AuthConfig{JWTSecret: "a-secret-with-at-least-32-bytes-long", PasswordPepper: "a-long-password-pepper", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour}
	server, healthServer := newGRPCServer(authservice.New(nil, nil, cfg, slog.New(slog.DiscardHandler)))
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
		t.Fatal("auth server did not stop")
	}
}

func TestServeReportsListenerFailure(t *testing.T) {
	wantErr := errors.New("accept failed")
	server, healthServer := newGRPCServer(authservice.New(nil, nil, config.AuthConfig{}, slog.New(slog.DiscardHandler)))
	err := serve(t.Context(), errorListener{err: wantErr}, server, healthServer, slog.New(slog.DiscardHandler))
	require.ErrorContains(t, err, "serve gRPC")
	require.ErrorIs(t, err, wantErr)
}

func TestServeAcceptsStoppedServer(t *testing.T) {
	server, healthServer := newGRPCServer(authservice.New(nil, nil, config.AuthConfig{}, slog.New(slog.DiscardHandler)))
	err := serve(t.Context(), errorListener{err: grpc.ErrServerStopped}, server, healthServer, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
}

type errorListener struct{ err error }

func (l errorListener) Accept() (net.Conn, error) { return nil, l.err }
func (errorListener) Close() error                { return nil }
func (errorListener) Addr() net.Addr              { return testAddr("error-listener") }

type testAddr string

func (a testAddr) Network() string { return string(a) }
func (a testAddr) String() string  { return string(a) }

func writeAuthConfig(t *testing.T, level, dsn string) string {
	t.Helper()
	t.Setenv("LOG_LEVEL", level)
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("AUTH_DB_DSN", dsn)
	t.Setenv("AUTH_JWT_SECRET", "a-secret-with-at-least-32-bytes-long")
	t.Setenv("AUTH_PASSWORD_PEPPER", "a-long-password-pepper")
	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "15m")
	t.Setenv("AUTH_REFRESH_TOKEN_TTL", "1h")
	path := filepath.Join(t.TempDir(), "auth.yaml")
	contents := []byte("db:\n  dsn: \"" + dsn + "\"\nlogger:\n  level: \"" + level + "\"\n  format: text\nauth:\n  jwt_secret: a-secret-with-at-least-32-bytes-long\n  password_pepper: a-long-password-pepper\n  access_token_ttl: 15m\n  refresh_token_ttl: 1h\ngrpc:\n  address: \":9001\"\n")
	require.NoError(t, os.WriteFile(path, contents, 0o600))
	return path
}
