package main

import (
	"log/slog"
	"net/http"

	"conduit/internal/config"
	authv1 "conduit/internal/gen/grpc/conduit/auth/v1"
	commentsv1 "conduit/internal/gen/grpc/conduit/comments/v1"
	postsv1 "conduit/internal/gen/grpc/conduit/posts/v1"
	profilev1 "conduit/internal/gen/grpc/conduit/profile/v1"
	subscriptionsv1 "conduit/internal/gen/grpc/conduit/subscriptions/v1"
	"conduit/internal/logger"
	"conduit/internal/metrics"
	authgrpc "conduit/internal/transport/grpc/auth"
	commentsgrpc "conduit/internal/transport/grpc/comments"
	"conduit/internal/transport/grpc/gateway"
	postsgrpc "conduit/internal/transport/grpc/posts"
	profilegrpc "conduit/internal/transport/grpc/profile"
	subscriptionsgrpc "conduit/internal/transport/grpc/subscriptions"
	httptransport "conduit/internal/transport/http"
	transportmiddleware "conduit/internal/transport/middleware"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type application struct {
	logger  *slog.Logger
	handler http.Handler
}

func newApplication(logger *slog.Logger, handler http.Handler) *application {
	return &application{logger: logger, handler: handler}
}

func provideLogger(cfg *config.Config) (*slog.Logger, error) {
	return logger.New(cfg.Logger)
}

func providePrometheusRegistry() *metrics.Registry {
	return metrics.NewRegistry()
}

func provideHTTPMetrics(registry *metrics.Registry) (*metrics.HTTP, error) {
	return metrics.NewHTTP(registry)
}

func providePanicReporter(logger *slog.Logger) metrics.PanicReporter {
	return func(r *http.Request, route string, recovered any, stack []byte) {
		logger.ErrorContext(
			r.Context(),
			"recovered from HTTP handler panic",
			"request_id", chimiddleware.GetReqID(r.Context()),
			"method", r.Method,
			"route", route,
			"panic", recovered,
			"stack", string(stack),
		)
	}
}

func provideApplicationService(cfg *config.Config) (httptransport.ApplicationService, func(), error) {
	targets := []string{
		cfg.Services.Auth.Target,
		cfg.Services.Profile.Target,
		cfg.Services.Posts.Target,
		cfg.Services.Comments.Target,
		cfg.Services.Subscriptions.Target,
	}
	connections := make([]*grpc.ClientConn, 0, len(targets))
	for _, target := range targets {
		connection, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			for _, opened := range connections {
				_ = opened.Close()
			}
			return nil, nil, err
		}
		connections = append(connections, connection)
	}
	cleanup := func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	}

	authClient := authgrpc.NewClient(authv1.NewAuthServiceClient(connections[0]))
	profileClient := profilegrpc.NewClient(profilev1.NewProfileServiceClient(connections[1]))
	subscriptionsClient := subscriptionsgrpc.NewClient(subscriptionsv1.NewSubscriptionsServiceClient(connections[4]))
	postsClient := postsgrpc.NewApplicationClient(postsv1.NewPostsServiceClient(connections[2]), profileClient, subscriptionsClient)
	commentsClient := commentsgrpc.NewApplicationClient(commentsv1.NewCommentsServiceClient(connections[3]), postsClient, profileClient, subscriptionsClient)
	return gateway.New(authClient, profileClient, subscriptionsClient, postsClient, commentsClient, cfg.Services.Timeout), cleanup, nil
}

func provideAuthMiddleware(svc httptransport.ApplicationService) *transportmiddleware.AuthMiddleware {
	return transportmiddleware.NewAuthMiddleware(svc)
}
