package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"conduit/internal/config"
	"conduit/internal/eventing"
	"conduit/internal/gen/postgres"
	"conduit/internal/metrics"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

type fakeOutbox struct{}

func (fakeOutbox) Claim(context.Context, int, time.Duration) ([]eventing.OutboxEvent, error) {
	return nil, nil
}
func (fakeOutbox) MarkPublished(context.Context, uuid.UUID) error                 { return nil }
func (fakeOutbox) Release(context.Context, uuid.UUID, time.Duration, error) error { return nil }

type fakeEventPublisher struct{}

func (fakeEventPublisher) Publish(context.Context, string, string, *eventing.Envelope) error {
	return nil
}

type fakeCloser struct{ closed bool }

func (f *fakeCloser) Close(context.Context) error { f.closed = true; return nil }

type fakePool struct{ closed bool }

func (f *fakePool) Close() { f.closed = true }
func (f *fakePool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *fakePool) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (f *fakePool) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }

func TestLoadConfig(t *testing.T) {
	setConfigEnvironment(t)
	path := writeRelayConfig(t)
	cfg, err := loadConfig([]string{"--config", path})
	require.NoError(t, err)
	require.Equal(t, "subscriptions", cfg.Relay.Source)
	_, err = loadConfig([]string{"--unknown"})
	require.ErrorContains(t, err, "parse flags")
	_, err = loadConfig([]string{"--config", filepath.Join(t.TempDir(), "missing")})
	require.ErrorContains(t, err, "read outbox relay config")
}

func TestRunWithLifecycleAndErrors(t *testing.T) {
	setConfigEnvironment(t)
	path := writeRelayConfig(t)
	relay, err := eventing.NewRelay(fakeOutbox{}, fakeEventPublisher{}, eventing.RelayConfig{
		BatchSize: 1, PollInterval: time.Millisecond, LockTimeout: time.Second,
		RetryDelay: time.Second, MaxRetryDelay: time.Second,
	}, nil, nil)
	require.NoError(t, err)
	pool, publisher := &fakePool{}, &fakeCloser{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err = runWith(ctx, []string{"--config", path}, func(context.Context, *config.OutboxRelayConfig) (*application, error) {
		return &application{
			relay: relay, closePublisher: publisher.Close, closeDatabase: pool.Close,
			handler: http.NewServeMux(), logger: slog.New(slog.DiscardHandler),
		}, nil
	}, net.Listen)
	require.NoError(t, err)
	require.True(t, pool.closed)
	require.True(t, publisher.closed)

	wantErr := errors.New("unavailable")
	err = runWith(t.Context(), []string{"--config", path}, func(context.Context, *config.OutboxRelayConfig) (*application, error) {
		return nil, wantErr
	}, net.Listen)
	require.ErrorIs(t, err, wantErr)

	pool, publisher = &fakePool{}, &fakeCloser{}
	err = runWith(t.Context(), []string{"--config", path}, func(context.Context, *config.OutboxRelayConfig) (*application, error) {
		return &application{closeDatabase: pool.Close, closePublisher: publisher.Close}, nil
	}, func(string, string) (net.Listener, error) { return nil, wantErr })
	require.ErrorIs(t, err, wantErr)
	require.True(t, pool.closed)
	require.True(t, publisher.closed)
}

func TestServeExposesHealth(t *testing.T) {
	relay, err := eventing.NewRelay(fakeOutbox{}, fakeEventPublisher{}, eventing.RelayConfig{
		BatchSize: 1, PollInterval: time.Millisecond, LockTimeout: time.Second,
		RetryDelay: time.Second, MaxRetryDelay: time.Second,
	}, nil, nil)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) })
	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	publisher := &fakeCloser{}
	app := &application{
		relay: relay, closePublisher: publisher.Close, closeDatabase: func() {},
		handler: mux, logger: slog.New(slog.DiscardHandler),
	}
	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() { errCh <- serve(ctx, listener, app, "test") }()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+listener.Addr().String()+"/healthz", nil)
	require.NoError(t, err)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusNoContent, response.StatusCode)
	cancel()
	require.NoError(t, <-errCh)
	require.True(t, publisher.closed)
}

func TestServeReturnsMetricsListenerFailure(t *testing.T) {
	relay, err := eventing.NewRelay(fakeOutbox{}, fakeEventPublisher{}, eventing.RelayConfig{
		BatchSize: 1, PollInterval: time.Millisecond, LockTimeout: time.Second,
		RetryDelay: time.Second, MaxRetryDelay: time.Second,
	}, nil, nil)
	require.NoError(t, err)
	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, listener.Close())
	publisher := &fakeCloser{}
	app := &application{
		relay: relay, closePublisher: publisher.Close, closeDatabase: func() {}, handler: http.NewServeMux(),
		logger: slog.New(slog.DiscardHandler),
	}

	err = serve(t.Context(), listener, app, "test")
	require.ErrorContains(t, err, "serve outbox metrics")
	require.True(t, publisher.closed)
}

func TestInitializeRejectsLoggerAndDatabase(t *testing.T) {
	cfg := validConfig()
	cfg.Logger.Level = "invalid"
	_, err := initialize(t.Context(), cfg)
	require.ErrorContains(t, err, "initialize logger")
	cfg = validConfig()
	cfg.DB.DSN = "://"
	_, err = initialize(t.Context(), cfg)
	require.ErrorContains(t, err, "initialize outbox database")
}

func TestNewApplicationBuildsHealthHandler(t *testing.T) {
	pool := &fakePool{}
	publisher := &fakeCloser{}
	relay, err := eventing.NewRelay(fakeOutbox{}, fakeEventPublisher{}, eventing.RelayConfig{
		BatchSize: 1, PollInterval: time.Millisecond, LockTimeout: time.Second,
		RetryDelay: time.Second, MaxRetryDelay: time.Second,
	}, nil, nil)
	require.NoError(t, err)
	app := newApplication(relay, publisher.Close, pool.Close, metrics.NewRegistry(), slog.New(slog.DiscardHandler))
	require.NotNil(t, app.relay)
	require.NotNil(t, app.handler)
	recorder := httptest.NewRecorder()
	app.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
}

type fakeCombinedPublisher struct{ fakeCloser }

func (*fakeCombinedPublisher) Publish(context.Context, string, string, *eventing.Envelope) error {
	return nil
}

func TestInitializeWithBuildsApplicationAndCleansUpFailures(t *testing.T) {
	pool := &fakePool{}
	publisher := &fakeCombinedPublisher{}
	app, err := initializeWith(t.Context(), validConfig(),
		func(context.Context, string, *slog.Logger) (postgres.DBTX, func(), error) {
			return pool, pool.Close, nil
		},
		func(*eventing.KafkaConfig, eventing.Observer) (eventing.Publisher, func(context.Context) error, error) {
			return publisher, publisher.Close, nil
		},
	)
	require.NoError(t, err)
	require.NotNil(t, app.relay)
	require.NotNil(t, app.handler)

	invalidRelay := validConfig()
	invalidRelay.Relay.BatchSize = 0
	pool = &fakePool{}
	publisher = &fakeCombinedPublisher{}
	_, err = initializeWith(t.Context(), invalidRelay,
		func(context.Context, string, *slog.Logger) (postgres.DBTX, func(), error) {
			return pool, pool.Close, nil
		},
		func(*eventing.KafkaConfig, eventing.Observer) (eventing.Publisher, func(context.Context) error, error) {
			return publisher, publisher.Close, nil
		},
	)
	require.ErrorContains(t, err, "batch size")
	require.True(t, pool.closed)
	require.True(t, publisher.closed)

	wantErr := errors.New("publisher")
	pool = &fakePool{}
	_, err = initializeWith(t.Context(), validConfig(),
		func(context.Context, string, *slog.Logger) (postgres.DBTX, func(), error) {
			return pool, pool.Close, nil
		},
		func(*eventing.KafkaConfig, eventing.Observer) (eventing.Publisher, func(context.Context) error, error) {
			return nil, nil, wantErr
		},
	)
	require.ErrorIs(t, err, wantErr)
	require.True(t, pool.closed)
}

func TestRunParsesArguments(t *testing.T) {
	err := run(t.Context(), []string{"--unknown"})
	require.ErrorContains(t, err, "parse flags")
}

func validConfig() *config.OutboxRelayConfig {
	return &config.OutboxRelayConfig{
		DB:     config.OutboxDBConfig{DSN: "postgres://localhost/outbox"},
		Logger: config.LoggerConfig{Level: "info", Format: "text"},
		Kafka:  config.OutboxKafkaConfig{Brokers: []string{"localhost:9092"}, ClientID: "relay"},
		Relay:  config.OutboxWorkerConfig{Source: "subscriptions", BatchSize: 1, PollInterval: time.Second, LockTimeout: time.Second, RetryDelay: time.Second, MaxRetryDelay: time.Second, MetricsAddress: "127.0.0.1:0"},
	}
}

func setConfigEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("OUTBOX_DB_DSN", "postgres://localhost/outbox")
	t.Setenv("KAFKA_BROKERS", "localhost:9092")
	t.Setenv("KAFKA_CLIENT_ID", "relay")
	t.Setenv("OUTBOX_SOURCE", "subscriptions")
	t.Setenv("OUTBOX_METRICS_ADDR", "127.0.0.1:0")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "text")
}

func writeRelayConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "outbox.yaml")
	contents := []byte("db:\n  dsn: postgres://localhost/outbox\nlogger:\n  level: info\n  format: text\nkafka:\n  brokers: [localhost:9092]\n  client_id: relay\nrelay:\n  source: subscriptions\n  batch_size: 1\n  poll_interval: 1ms\n  lock_timeout: 1s\n  retry_delay: 1s\n  max_retry_delay: 1s\n  metrics_address: 127.0.0.1:0\n")
	require.NoError(t, os.WriteFile(path, contents, 0o600))
	return path
}
