package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"conduit/internal/config"
	"conduit/internal/metrics"
	"conduit/internal/transport/grpc/gateway"
	httptransport "conduit/internal/transport/http"
	transportmiddleware "conduit/internal/transport/middleware"

	"github.com/stretchr/testify/require"
)

func gatewayConfig() *config.Config {
	return &config.Config{
		Logger: config.LoggerConfig{Level: "info", Format: "text"},
		HTTP:   config.HTTPConfig{AllowedOrigins: []string{"*"}},
		Services: config.RemoteServicesConfig{
			Timeout:       time.Second,
			Auth:          config.AuthGRPCClientConfig{Target: "dns:///auth:9001"},
			Profile:       config.ProfileGRPCClientConfig{Target: "dns:///profile:9002"},
			Posts:         config.PostsGRPCClientConfig{Target: "dns:///posts:9003"},
			Comments:      config.CommentsGRPCClientConfig{Target: "dns:///comments:9005"},
			Subscriptions: config.SubscriptionsGRPCClientConfig{Target: "dns:///subscriptions:9004"},
		},
	}
}

func TestProviders(t *testing.T) {
	cfg := gatewayConfig()
	appLogger, err := provideLogger(cfg)
	require.NoError(t, err)

	app := newApplication(appLogger, http.NotFoundHandler())
	require.Equal(t, appLogger, app.logger)
	require.NotNil(t, app.handler)

	registry := providePrometheusRegistry()
	httpMetrics, err := provideHTTPMetrics(registry)
	require.NoError(t, err)
	require.NotNil(t, httpMetrics)

	reporter := providePanicReporter(slog.New(slog.DiscardHandler))
	require.NotPanics(t, func() {
		reporter(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", nil), "/panic", "boom", []byte("stack"))
	})

	remote, cleanup, err := provideApplicationService(cfg)
	require.NoError(t, err)
	require.NotNil(t, remote)
	require.NotNil(t, provideAuthMiddleware(remote))
	cleanup()
}

func TestProvideApplicationServiceRejectsInvalidTarget(t *testing.T) {
	cfg := gatewayConfig()
	cfg.Services.Profile.Target = "%"
	remote, cleanup, err := provideApplicationService(cfg)
	require.Error(t, err)
	require.Nil(t, remote)
	require.Nil(t, cleanup)
}

func TestProvideHTTPHandler(t *testing.T) {
	registry := metrics.NewRegistry()
	httpMetrics, err := metrics.NewHTTP(registry)
	require.NoError(t, err)
	facade := gateway.New(nil, nil, nil, nil, nil, time.Second)
	handler, err := provideHTTPHandler(
		gatewayConfig(),
		transportmiddleware.NewAuthMiddleware(facade),
		httpMetrics,
		providePanicReporter(slog.New(slog.DiscardHandler)),
		httptransport.NewServer(facade),
		facade,
		registry,
	)
	require.NoError(t, err)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{name: "metrics", method: http.MethodGet, path: "/metrics", status: http.StatusOK},
		{name: "missing authentication", method: http.MethodGet, path: "/api/user", status: http.StatusUnauthorized},
		{name: "invalid request", method: http.MethodPost, path: "/api/users", body: `{`, status: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, io.NopCloser(strings.NewReader(tt.body)))
			if tt.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			require.Equal(t, tt.status, recorder.Code)
		})
	}

	recorder := httptest.NewRecorder()
	writeError(recorder, http.StatusTeapot, "body", "teapot")
	require.Equal(t, http.StatusTeapot, recorder.Code)
	require.Contains(t, recorder.Body.String(), "teapot")
}

func TestRunReportsConfigurationError(t *testing.T) {
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	t.Cleanup(func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	})
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"server", "-config", filepath.Join(t.TempDir(), "missing.yaml")}

	err := run()
	require.ErrorContains(t, err, "initialize application")
}

func TestRunWithReportsInitializationAndServeErrors(t *testing.T) {
	wantErr := errors.New("unavailable")
	err := runWith(t.Context(), "config.yaml", "127.0.0.1:0", func(context.Context, string) (*application, func(), error) {
		return nil, nil, wantErr
	}, func(*http.Server) error { return nil }, func(*http.Server, context.Context) error { return nil })
	require.ErrorIs(t, err, wantErr)

	cleaned := false
	app := &application{logger: slog.New(slog.DiscardHandler), handler: http.NotFoundHandler()}
	err = runWith(t.Context(), "config.yaml", "127.0.0.1:0", func(context.Context, string) (*application, func(), error) {
		return app, func() { cleaned = true }, nil
	}, func(*http.Server) error { return wantErr }, func(*http.Server, context.Context) error { return nil })
	require.ErrorIs(t, err, wantErr)
	require.True(t, cleaned)
}

func TestRunWithGracefulCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	cleaned := false
	app := &application{logger: slog.New(slog.DiscardHandler), handler: http.NotFoundHandler()}
	err = runWith(ctx, "config.yaml", "127.0.0.1:0", func(context.Context, string) (*application, func(), error) {
		return app, func() { cleaned = true }, nil
	}, func(server *http.Server) error { return server.Serve(listener) }, func(server *http.Server, ctx context.Context) error { return server.Shutdown(ctx) })
	require.NoError(t, err)
	require.True(t, cleaned)
}

func TestRunWithAcceptsServerClosed(t *testing.T) {
	app := &application{logger: slog.New(slog.DiscardHandler), handler: http.NotFoundHandler()}
	err := runWith(t.Context(), "config.yaml", "127.0.0.1:0", func(context.Context, string) (*application, func(), error) {
		return app, func() {}, nil
	}, func(*http.Server) error { return http.ErrServerClosed }, func(*http.Server, context.Context) error { return nil })
	require.NoError(t, err)
}

func TestRunWithShutdownFailures(t *testing.T) {
	wantErr := errors.New("shutdown failed")
	serveErr := errors.New("serve failed")
	app := &application{logger: slog.New(slog.DiscardHandler), handler: http.NotFoundHandler()}
	for name, shutdownErr := range map[string]error{"shutdown": wantErr, "serve during shutdown": nil} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			release := make(chan struct{})
			err := runWith(ctx, "config.yaml", "127.0.0.1:0", func(context.Context, string) (*application, func(), error) {
				return app, func() {}, nil
			}, func(*http.Server) error {
				<-release
				if shutdownErr != nil {
					return http.ErrServerClosed
				}
				return serveErr
			}, func(*http.Server, context.Context) error {
				close(release)
				return shutdownErr
			})
			if shutdownErr != nil {
				require.ErrorIs(t, err, wantErr)
				return
			}
			require.ErrorIs(t, err, serveErr)
		})
	}
}
