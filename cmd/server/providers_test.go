package main

import (
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"conduit/internal/config"
	"conduit/internal/gen/postgres"
	"conduit/internal/service"
	httptransport "conduit/internal/transport/http"
	transportmiddleware "conduit/internal/transport/middleware"

	"github.com/prometheus/client_golang/prometheus"
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
	followService := provideFollowService(querier)
	favoriteService := provideFavoriteService(querier)
	articleTagService := provideArticleTagService(querier)
	userService := provideUserService(querier, followService, cfg)
	require.NotNil(t, provideArticleService(querier, transactions, userService, provideTagService(querier), articleTagService, favoriteService, followService))
	require.NotNil(t, provideAuthService(querier, transactions, cfg))
	require.NotNil(t, provideCommentService(querier, userService))
	require.NotNil(t, followService)
	require.NotNil(t, favoriteService)
	require.NotNil(t, articleTagService)
	require.NotNil(t, userService)
	require.NotNil(t, provideTagService(querier))

	reporter := providePanicReporter(slog.New(slog.DiscardHandler))
	require.NotPanics(t, func() {
		reporter(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", nil), "/panic", "boom", []byte("stack"))
	})
}

func TestProvideDatabaseRejectsInvalidDSN(t *testing.T) {
	pool, cleanup, err := provideDatabase(t.Context(), &config.Config{DB: config.DBConfig{DSN: "://"}}, slog.Default())
	require.Error(t, err)
	require.Nil(t, pool)
	require.Nil(t, cleanup)
}

func TestProvideHTTPHandler(t *testing.T) {
	registry := prometheus.NewRegistry()
	httpMetrics, err := transportmiddleware.NewHTTPMetrics(registry)
	require.NoError(t, err)
	facade := service.New(nil, nil, nil, nil, nil)
	handler, err := provideHTTPHandler(
		&config.Config{HTTP: config.HTTPConfig{AllowedOrigins: []string{"*"}}},
		slog.New(slog.DiscardHandler),
		transportmiddleware.NewAuthMiddleware(facade),
		httpMetrics,
		providePanicReporter(slog.New(slog.DiscardHandler)),
		httptransport.NewServer(facade),
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

type providerQuerierStub struct{ postgres.Querier }
