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
	"conduit/internal/gen/postgres"
	"conduit/internal/metrics"
	"conduit/internal/service"
	httptransport "conduit/internal/transport/http"
	transportmiddleware "conduit/internal/transport/middleware"

	"github.com/stretchr/testify/require"
)

func TestSimpleProviders(t *testing.T) {
	cfg := &config.Config{
		Logger: config.LoggerConfig{Level: "info", Format: "text"},
		Auth: config.AuthConfig{
			JWTSecret:       "test-secret-that-is-at-least-32-bytes",
			PasswordPepper:  "password-pepper-long-enough",
			AccessTokenTTL:  time.Minute,
			RefreshTokenTTL: time.Hour,
		},
	}
	logger, err := provideLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)

	app := newApplication(logger, http.NotFoundHandler())
	require.Equal(t, logger, app.logger)
	require.NotNil(t, app.handler)

	registry := providePrometheusRegistry()
	require.NotNil(t, registry)
	repositoryMetrics, err := provideRepositoryMetrics(registry)
	require.NoError(t, err)
	httpMetrics, err := provideHTTPMetrics(registry)
	require.NoError(t, err)
	require.NotNil(t, httpMetrics)

	querier := providerQuerierStub{}
	require.NotNil(t, provideQueries(nil, repositoryMetrics))
	require.NotNil(t, provideQueryDecorator(repositoryMetrics)(querier))
	transactions := provideTransactions(nil, func(value postgres.Querier) postgres.Querier { return value })
	followService, closeFollows := provideFollowService(querier, cfg, logger)
	t.Cleanup(closeFollows)
	favoriteService := provideFavoriteService(querier, logger)
	articleTagService := provideArticleTagService(querier, logger)
	userService := provideUserService(querier, followService, cfg, logger)
	require.NotNil(t, provideArticleService(querier, transactions, userService, provideTagService(querier, logger), articleTagService, favoriteService, followService, logger))
	require.NotNil(t, provideAuthService(querier, transactions, cfg, logger))
	require.NotNil(t, provideCommentService(querier, userService, logger))
	require.NotNil(t, followService)
	require.NotNil(t, favoriteService)
	require.NotNil(t, articleTagService)
	require.NotNil(t, userService)
	require.NotNil(t, provideTagService(querier, logger))

	reporter := providePanicReporter(slog.New(slog.DiscardHandler))
	require.NotPanics(t, func() {
		reporter(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", nil), "/panic", "boom", []byte("stack"))
	})
}

func TestProvideRemoteFollowService(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Services: config.RemoteServicesConfig{
		Subscriptions: config.GRPCClientConfig{Target: "dns:///localhost:9004"},
	}}
	dependency, cleanup := provideFollowService(providerQuerierStub{}, cfg, slog.Default())
	require.NotNil(t, dependency)
	require.NotNil(t, cleanup)
	cleanup()
}

func TestProvideApplicationServiceModes(t *testing.T) {
	local, cleanup, err := provideApplicationService(nil, nil, nil, nil, nil, &config.Config{Services: config.RemoteServicesConfig{Mode: "local"}})
	require.NoError(t, err)
	require.NotNil(t, local)
	cleanup()

	remote, cleanup, err := provideApplicationService(nil, nil, nil, nil, nil, &config.Config{Services: config.RemoteServicesConfig{
		Mode: "grpc", Timeout: time.Second,
		Auth: config.AuthGRPCClientConfig{Target: "dns:///auth:9001"}, Profile: config.ProfileGRPCClientConfig{Target: "dns:///profile:9002"},
		Posts: config.PostsGRPCClientConfig{Target: "dns:///posts:9003"}, Comments: config.CommentsGRPCClientConfig{Target: "dns:///comments:9005"},
		Subscriptions: config.SubscriptionsGRPCClientConfig{Target: "dns:///subscriptions:9004"},
	}})
	require.NoError(t, err)
	require.NotNil(t, remote)
	cleanup()

	remote, cleanup, err = provideApplicationService(nil, nil, nil, nil, nil, &config.Config{Services: config.RemoteServicesConfig{
		Mode: "grpc", Auth: config.AuthGRPCClientConfig{Target: "dns:///auth:9001"}, Profile: config.ProfileGRPCClientConfig{Target: "%"},
	}})
	require.Error(t, err)
	require.Nil(t, remote)
	require.Nil(t, cleanup)
}

func TestProvideDatabaseRejectsInvalidDSN(t *testing.T) {
	pool, cleanup, err := provideDatabase(t.Context(), &config.Config{DB: config.DBConfig{DSN: "://"}}, slog.Default())
	require.Error(t, err)
	require.Nil(t, pool)
	require.Nil(t, cleanup)
}

func TestProvideHTTPHandler(t *testing.T) {
	registry := metrics.NewRegistry()
	httpMetrics, err := metrics.NewHTTP(registry)
	require.NoError(t, err)
	facade := service.New(nil, nil, nil, nil, nil)
	require.NotNil(t, provideAuthMiddleware(facade))
	handler, err := provideHTTPHandler(
		&config.Config{HTTP: config.HTTPConfig{AllowedOrigins: []string{"*"}}},
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

type providerQuerierStub struct{ postgres.Querier }
