package main

import (
	"context"
	"log/slog"
	"net/http"

	"conduit/internal/config"
	subscriptionsv1 "conduit/internal/gen/grpc/conduit/subscriptions/v1"
	"conduit/internal/gen/postgres"
	"conduit/internal/logger"
	"conduit/internal/metrics"
	"conduit/internal/repository/transaction"
	"conduit/internal/service/article"
	"conduit/internal/service/articletag"
	"conduit/internal/service/auth"
	"conduit/internal/service/comment"
	"conduit/internal/service/favorite"
	"conduit/internal/service/follow"
	"conduit/internal/service/tag"
	"conduit/internal/service/user"
	pgstorage "conduit/internal/storage/postgres"
	subscriptionsgrpc "conduit/internal/transport/grpc/subscriptions"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
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

func provideDatabase(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*pgxpool.Pool, func(), error) {
	pool, err := pgstorage.New(ctx, cfg.DB.DSN, logger)
	if err != nil {
		return nil, nil, err
	}
	return pool, pool.Close, nil
}

func providePrometheusRegistry() *metrics.Registry {
	return metrics.NewRegistry()
}

func provideRepositoryMetrics(registry *metrics.Registry) (*metrics.Repository, error) {
	return metrics.NewRepository(registry)
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

func provideQueries(pool *pgxpool.Pool, repositoryMetrics *metrics.Repository) postgres.Querier {
	return repositoryMetrics.Wrap(postgres.New(pool))
}

func provideQueryDecorator(repositoryMetrics *metrics.Repository) func(postgres.Querier) postgres.Querier {
	return repositoryMetrics.Wrap
}

func provideTransactions(
	pool *pgxpool.Pool,
	decorate func(postgres.Querier) postgres.Querier,
) *transaction.Transactions {
	return transaction.New(pool, decorate)
}

func provideArticleService(
	repository postgres.Querier,
	transactions *transaction.Transactions,
	users *user.Service,
	tags *tag.Service,
	articleTags *articletag.Service,
	favorites *favorite.Service,
	follows follow.Dependency,
	logger *slog.Logger,
) *article.Service {
	return article.New(repository, transactions, users, tags, articleTags, favorites, follows, logger)
}

func provideAuthService(queries postgres.Querier, transactions *transaction.Transactions, cfg *config.Config, logger *slog.Logger) *auth.Service {
	return auth.New(queries, transactions, cfg.Auth, logger)
}

func provideFollowService(queries postgres.Querier, cfg *config.Config, logger *slog.Logger) (follow.Dependency, func(), error) {
	if cfg.Services.Subscriptions.Target == "" {
		return follow.New(queries, logger), func() {}, nil
	}
	connection, err := grpc.NewClient(
		cfg.Services.Subscriptions.Target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}
	return subscriptionsgrpc.NewClient(subscriptionsv1.NewSubscriptionsServiceClient(connection)), func() { _ = connection.Close() }, nil
}

func provideFavoriteService(queries postgres.Querier, logger *slog.Logger) *favorite.Service {
	return favorite.New(queries, logger)
}

func provideArticleTagService(queries postgres.Querier, logger *slog.Logger) *articletag.Service {
	return articletag.New(queries, logger)
}

func provideUserService(queries postgres.Querier, follows follow.Dependency, cfg *config.Config, logger *slog.Logger) *user.Service {
	return user.New(queries, follows, auth.NewPasswordManager(cfg.Auth.PasswordPepper), logger)
}

func provideCommentService(queries postgres.Querier, users *user.Service, logger *slog.Logger) *comment.Service {
	return comment.New(queries, users, logger)
}

func provideTagService(queries postgres.Querier, logger *slog.Logger) *tag.Service {
	return tag.New(queries, logger)
}
