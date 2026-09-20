package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"conduit/internal/config"
	"conduit/internal/eventing"
	"conduit/internal/gen/postgres"
	"conduit/internal/logger"
	"conduit/internal/metrics"
	pgstorage "conduit/internal/storage/postgres"
)

const (
	defaultConfigPath = "config/outbox-relay.yaml"
	shutdownTimeout   = 10 * time.Second
)

type application struct {
	relay          *eventing.Relay
	closePublisher func(context.Context) error
	closeDatabase  func()
	handler        http.Handler
	logger         *slog.Logger
}

type poolFactory func(context.Context, string, *slog.Logger) (postgres.DBTX, func(), error)
type publisherFactory func(*eventing.KafkaConfig, eventing.Observer) (eventing.Publisher, func(context.Context) error, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:])
	stop()
	if err != nil {
		log.Printf("outbox relay: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	return runWith(ctx, args, initialize, net.Listen)
}

type appInitializer func(context.Context, *config.OutboxRelayConfig) (*application, error)
type listenerFactory func(string, string) (net.Listener, error)

func runWith(ctx context.Context, args []string, initializeApp appInitializer, listen listenerFactory) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return err
	}
	app, err := initializeApp(ctx, cfg)
	if err != nil {
		return err
	}
	defer app.closeDatabase()

	listener, err := listen("tcp", cfg.Relay.MetricsAddress)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		return errors.Join(fmt.Errorf("listen for metrics: %w", err), app.closePublisher(shutdownCtx))
	}
	defer listener.Close()
	return serve(ctx, listener, app, cfg.Relay.Source)
}

func initialize(ctx context.Context, cfg *config.OutboxRelayConfig) (*application, error) {
	return initializeWith(ctx, cfg,
		func(ctx context.Context, dsn string, appLogger *slog.Logger) (postgres.DBTX, func(), error) {
			pool, err := pgstorage.New(ctx, dsn, appLogger)
			if err != nil {
				return nil, nil, err
			}
			return pool, pool.Close, nil
		},
		func(kafkaConfig *eventing.KafkaConfig, observer eventing.Observer) (eventing.Publisher, func(context.Context) error, error) {
			publisher, err := eventing.NewKafkaPublisher(kafkaConfig, observer)
			if err != nil {
				return nil, nil, err
			}
			return publisher, publisher.Close, nil
		},
	)
}

func initializeWith(ctx context.Context, cfg *config.OutboxRelayConfig, newPool poolFactory, newPublisher publisherFactory) (*application, error) {
	appLogger, err := logger.New(cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}
	database, closeDatabase, err := newPool(ctx, cfg.DB.DSN, appLogger)
	if err != nil {
		return nil, fmt.Errorf("initialize outbox database: %w", err)
	}
	registry := metrics.NewRegistry()
	eventMetrics, err := eventing.NewMetrics(registry)
	if err != nil {
		closeDatabase()
		return nil, err
	}
	publisher, closePublisher, err := newPublisher(&eventing.KafkaConfig{
		Brokers: cfg.Kafka.Brokers, ClientID: cfg.Kafka.ClientID,
		Username: cfg.Kafka.Username, Password: cfg.Kafka.Password, TLS: cfg.Kafka.TLS,
	}, eventMetrics)
	if err != nil {
		closeDatabase()
		return nil, err
	}
	relay, err := eventing.NewRelay(
		eventing.NewOutboxStore(postgres.New(database)), publisher,
		eventing.RelayConfig{
			BatchSize: cfg.Relay.BatchSize, PollInterval: cfg.Relay.PollInterval,
			LockTimeout: cfg.Relay.LockTimeout, RetryDelay: cfg.Relay.RetryDelay,
			MaxRetryDelay: cfg.Relay.MaxRetryDelay,
		}, eventMetrics, appLogger,
	)
	if err != nil {
		_ = closePublisher(ctx)
		closeDatabase()
		return nil, err
	}
	return newApplication(relay, closePublisher, closeDatabase, registry, appLogger), nil
}

func newApplication(
	relay *eventing.Relay,
	closePublisher func(context.Context) error,
	closeDatabase func(),
	registry *metrics.Registry,
	appLogger *slog.Logger,
) *application {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", metrics.ScrapeHandler(registry))
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})
	return &application{
		relay: relay, closePublisher: closePublisher, closeDatabase: closeDatabase,
		handler: mux, logger: appLogger,
	}
}

func serve(ctx context.Context, listener net.Listener, app *application, source string) error {
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	server := &http.Server{
		Handler: app.handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second,
	}
	relayErr := make(chan error, 1)
	go func() { relayErr <- app.relay.Run(workerCtx) }()
	httpErr := make(chan error, 1)
	go func() { httpErr <- server.Serve(listener) }()
	app.logger.InfoContext(ctx, "outbox relay initialized", "source", source, "metrics_addr", listener.Addr().String())

	var runErr error
	relayStopped := false
	select {
	case <-ctx.Done():
	case err := <-relayErr:
		runErr = err
		relayStopped = true
	case err := <-httpErr:
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("serve outbox metrics: %w", err)
		}
	}
	cancel()
	if !relayStopped {
		runErr = errors.Join(runErr, <-relayErr)
	}
	app.logger.InfoContext(context.WithoutCancel(ctx), "shutting down outbox relay", "source", source)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	closeErr := app.closePublisher(shutdownCtx)
	return errors.Join(runErr, shutdownErr, closeErr)
}

func loadConfig(args []string) (*config.OutboxRelayConfig, error) {
	flags := flag.NewFlagSet("outbox-relay", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", defaultConfigPath, "path to config file")
	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}
	return config.LoadOutboxRelay(*configPath)
}
