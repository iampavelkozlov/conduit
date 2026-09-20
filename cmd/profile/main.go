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
	"os"
	"os/signal"
	"syscall"
	"time"

	profilecache "conduit/internal/cache/profile"
	"conduit/internal/config"
	profilev1 "conduit/internal/gen/grpc/conduit/profile/v1"
	profilepostgres "conduit/internal/gen/profile/postgres"
	"conduit/internal/logger"
	profiledomain "conduit/internal/service/profile"
	pgstorage "conduit/internal/storage/postgres"
	profilegrpc "conduit/internal/transport/grpc/profile"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	defaultConfigPath = "config/profile.yaml"
	shutdownTimeout   = 10 * time.Second
	serviceName       = "conduit.profile.v1.ProfileService"
)

func main() {
	if err := mainRun(); err != nil {
		log.Printf("profile: %v", err)
		os.Exit(1)
	}
}

func mainRun() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, os.Args[1:])
}

func run(ctx context.Context, args []string) error {
	return runWith(ctx, args, initialize, new(net.ListenConfig).Listen)
}

type serviceInitializer func(context.Context, *config.ProfileConfig, *slog.Logger) (*grpc.Server, *health.Server, func(), error)
type listenerFactory func(context.Context, string, string) (net.Listener, error)

func runWith(ctx context.Context, args []string, initialize serviceInitializer, listen listenerFactory) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return err
	}
	appLogger, err := logger.New(cfg.Logger)
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	server, healthServer, cleanup, err := initialize(ctx, cfg, appLogger)
	if err != nil {
		return err
	}
	defer cleanup()
	listener, err := listen(ctx, "tcp", cfg.GRPC.Address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.GRPC.Address, err)
	}
	defer listener.Close()
	appLogger.InfoContext(ctx, "profile server initialized", "addr", cfg.GRPC.Address)
	return serve(ctx, listener, server, healthServer, appLogger)
}

func loadConfig(args []string) (*config.ProfileConfig, error) {
	flags := flag.NewFlagSet("profile", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", defaultConfigPath, "path to config file")
	address := flags.String("addr", "", "gRPC listen address (overrides config)")
	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}
	cfg, err := config.LoadProfile(*configPath)
	if err != nil {
		return nil, err
	}
	if *address != "" {
		cfg.GRPC.Address = *address
	}
	return cfg, nil
}

func initialize(ctx context.Context, cfg *config.ProfileConfig, appLogger *slog.Logger) (*grpc.Server, *health.Server, func(), error) {
	pool, err := pgstorage.New(ctx, cfg.DB.DSN, appLogger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("initialize profile database: %w", err)
	}
	redisCache := profilecache.New(cfg.Redis.Address, cfg.Redis.Password, cfg.Redis.DB, cfg.Redis.TTL)
	var cache profiledomain.Cache = redisCache
	service := profiledomain.New(profilepostgres.New(pool), cache, appLogger)
	server, healthServer := newGRPCServer(profilegrpc.NewServer(service))
	cleanup := func() {
		if err := redisCache.Close(); err != nil {
			appLogger.WarnContext(context.WithoutCancel(ctx), "close profile cache", "error", err)
		}
		pool.Close()
	}
	return server, healthServer, cleanup, nil
}

func newGRPCServer(profileServer profilev1.ProfileServiceServer) (*grpc.Server, *health.Server) {
	server := grpc.NewServer()
	profilev1.RegisterProfileServiceServer(server, profileServer)
	healthServer := health.NewServer()
	healthServer.SetServingStatus(serviceName, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	reflection.Register(server)
	return server, healthServer
}

func serve(ctx context.Context, listener net.Listener, server *grpc.Server, healthServer *health.Server, appLogger *slog.Logger) error {
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Serve(listener) }()
	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("serve gRPC: %w", err)
		}
		return nil
	case <-ctx.Done():
		appLogger.InfoContext(context.WithoutCancel(ctx), "shutting down profile server")
	}
	healthServer.SetServingStatus(serviceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()
	timer := time.NewTimer(shutdownTimeout)
	defer timer.Stop()
	select {
	case <-stopped:
	case <-timer.C:
		server.Stop()
		<-stopped
	}
	if err := <-serverErr; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("serve gRPC during shutdown: %w", err)
	}
	return nil
}
