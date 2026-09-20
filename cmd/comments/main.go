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
	"conduit/internal/gen/postgres"
	"conduit/internal/logger"
	"conduit/internal/service/comment"
	pgstorage "conduit/internal/storage/postgres"
	commentsgrpc "conduit/internal/transport/grpc/comments"
	postsgrpc "conduit/internal/transport/grpc/posts"
	profilegrpc "conduit/internal/transport/grpc/profile"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	defaultConfigPath = "config/comments.yaml"
	shutdownTimeout   = 10 * time.Second
	serviceName       = "conduit.comments.v1.CommentsService"
)

func main() {
	if err := mainRun(); err != nil {
		log.Printf("comments: %v", err)
		os.Exit(1)
	}
}

func mainRun() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, os.Args[1:])
}

func run(ctx context.Context, args []string) error {
	return runWith(ctx, args, initializeCommentService, net.Listen)
}

type serviceInitializer func(context.Context, *config.CommentsConfig, *slog.Logger) (*comment.Service, func(), error)
type listenerFactory func(string, string) (net.Listener, error)

func runWith(ctx context.Context, args []string, initialize serviceInitializer, listen listenerFactory) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return err
	}
	appLogger, err := logger.New(cfg.Logger)
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	commentService, cleanup, err := initialize(ctx, cfg, appLogger)
	if err != nil {
		return fmt.Errorf("initialize comments service: %w", err)
	}
	defer cleanup()

	listener, err := listen("tcp", cfg.GRPC.Address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.GRPC.Address, err)
	}
	defer listener.Close()

	server, healthServer := newGRPCServer(commentService)
	appLogger.InfoContext(ctx, "comments server initialized", "addr", cfg.GRPC.Address)
	return serve(ctx, listener, server, healthServer, appLogger)
}

func initializeCommentService(ctx context.Context, cfg *config.CommentsConfig, appLogger *slog.Logger) (*comment.Service, func(), error) {
	pool, err := pgstorage.New(ctx, cfg.DB.DSN, appLogger)
	if err != nil {
		return nil, nil, err
	}
	postsConn, err := grpc.NewClient(cfg.Clients.PostsAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("connect posts: %w", err)
	}
	profileConn, err := grpc.NewClient(cfg.Clients.ProfileAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = postsConn.Close()
		pool.Close()
		return nil, nil, fmt.Errorf("connect profile: %w", err)
	}

	var articles comment.ArticleResolver = postsgrpc.NewClient(postsv1.NewPostsServiceClient(postsConn))
	var profiles comment.ProfileReader = profilegrpc.NewClient(profilev1.NewProfileServiceClient(profileConn))
	cleanup := func() {
		_ = profileConn.Close()
		_ = postsConn.Close()
		pool.Close()
	}
	return comment.New(postgres.New(pool), profiles, articles, appLogger), cleanup, nil
}

func loadConfig(args []string) (*config.CommentsConfig, error) {
	flags := flag.NewFlagSet("comments", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", defaultConfigPath, "path to config file")
	address := flags.String("addr", "", "gRPC listen address (overrides config)")
	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}
	cfg, err := config.LoadComments(*configPath)
	if err != nil {
		return nil, err
	}
	if *address != "" {
		cfg.GRPC.Address = *address
	}
	return cfg, nil
}

func newGRPCServer(commentService *comment.Service) (*grpc.Server, *health.Server) {
	server := grpc.NewServer()
	commentsgrpc.Register(server, commentService)
	healthServer := health.NewServer()
	healthServer.SetServingStatus(serviceName, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	reflection.Register(server)
	return server, healthServer
}

func serve(ctx context.Context, listener net.Listener, server *grpc.Server, healthServer *health.Server, appLogger *slog.Logger) error {
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("serve gRPC: %w", err)
		}
		return nil
	case <-ctx.Done():
		appLogger.InfoContext(context.WithoutCancel(ctx), "shutting down comments server")
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
