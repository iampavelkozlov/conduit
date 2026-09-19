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

	"conduit/internal/config"
	postsv1 "conduit/internal/gen/grpc/conduit/posts/v1"
	profilev1 "conduit/internal/gen/grpc/conduit/profile/v1"
	subscriptionsv1 "conduit/internal/gen/grpc/conduit/subscriptions/v1"
	"conduit/internal/gen/postgres"
	"conduit/internal/logger"
	"conduit/internal/repository/transaction"
	"conduit/internal/service/article"
	"conduit/internal/service/articletag"
	"conduit/internal/service/favorite"
	"conduit/internal/service/tag"
	pgstorage "conduit/internal/storage/postgres"
	postsgrpc "conduit/internal/transport/grpc/posts"
	profilegrpc "conduit/internal/transport/grpc/profile"
	subscriptionsgrpc "conduit/internal/transport/grpc/subscriptions"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	defaultConfigPath = "config/posts.yaml"
	shutdownTimeout   = 10 * time.Second
	serviceName       = "conduit.posts.v1.PostsService"
)

func main() {
	if err := mainRun(); err != nil {
		log.Printf("posts: %v", err)
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

type serviceInitializer func(context.Context, *config.PostsConfig, *slog.Logger) (*grpc.Server, *health.Server, func(), error)
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

	appLogger.InfoContext(ctx, "posts server initialized", "addr", cfg.GRPC.Address)
	return serve(ctx, listener, server, healthServer, appLogger)
}

func loadConfig(args []string) (*config.PostsConfig, error) {
	flags := flag.NewFlagSet("posts", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", defaultConfigPath, "path to config file")
	address := flags.String("addr", "", "gRPC listen address (overrides config)")
	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}
	cfg, err := config.LoadPosts(*configPath)
	if err != nil {
		return nil, err
	}
	if *address != "" {
		cfg.GRPC.Address = *address
	}
	return cfg, nil
}

func initialize(ctx context.Context, cfg *config.PostsConfig, appLogger *slog.Logger) (*grpc.Server, *health.Server, func(), error) {
	pool, err := pgstorage.New(ctx, cfg.DB.DSN, appLogger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("initialize posts database: %w", err)
	}
	profileConnection, err := grpc.NewClient(cfg.Profile.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("initialize profile client: %w", err)
	}
	subscriptionsConnection, err := grpc.NewClient(cfg.Subscriptions.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = profileConnection.Close()
		pool.Close()
		return nil, nil, nil, fmt.Errorf("initialize subscriptions client: %w", err)
	}
	cleanup := func() {
		_ = errors.Join(profileConnection.Close(), subscriptionsConnection.Close())
		pool.Close()
	}

	queries := postgres.New(pool)
	transactions := transaction.New(pool, func(querier postgres.Querier) postgres.Querier { return querier })
	var profiles article.UserService = profilegrpc.NewClient(profilev1.NewProfileServiceClient(profileConnection))
	var subscriptions article.FollowService = subscriptionsgrpc.NewClient(subscriptionsv1.NewSubscriptionsServiceClient(subscriptionsConnection))
	tags := tag.New(queries, appLogger)
	articleTags := articletag.New(queries, appLogger)
	favorites := favorite.New(queries, appLogger)
	articles := article.New(queries, transactions, profiles, tags, articleTags, favorites, subscriptions, appLogger)

	server, healthServer := newGRPCServer(postsgrpc.NewServer(articles, tags))
	return server, healthServer, cleanup, nil
}

func newGRPCServer(postsServer postsv1.PostsServiceServer) (*grpc.Server, *health.Server) {
	server := grpc.NewServer()
	postsv1.RegisterPostsServiceServer(server, postsServer)
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
		appLogger.InfoContext(context.WithoutCancel(ctx), "shutting down posts server")
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
