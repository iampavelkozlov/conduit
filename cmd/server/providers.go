package main

import (
	"context"
	"log/slog"
	"net/http"

	"conduit/internal/config"
	"conduit/internal/gen/postgres"
	"conduit/internal/logger"
	repositorymetrics "conduit/internal/repository/metrics"
	"conduit/internal/service/article"
	"conduit/internal/service/articletag"
	"conduit/internal/service/auth"
	"conduit/internal/service/comment"
	"conduit/internal/service/favorite"
	"conduit/internal/service/follow"
	"conduit/internal/service/tag"
	"conduit/internal/service/user"
	pgstorage "conduit/internal/storage/postgres"
	transportmiddleware "conduit/internal/transport/middleware"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
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

func providePrometheusRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return registry
}

func provideRepositoryMetrics(registry *prometheus.Registry) (*repositorymetrics.Metrics, error) {
	return repositorymetrics.New(registry)
}

func provideHTTPMetrics(registry *prometheus.Registry) (*transportmiddleware.HTTPMetrics, error) {
	return transportmiddleware.NewHTTPMetrics(registry)
}

func providePanicReporter(logger *slog.Logger) transportmiddleware.PanicReporter {
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

func provideQueries(pool *pgxpool.Pool, metrics *repositorymetrics.Metrics) postgres.Querier {
	return metrics.Wrap(postgres.New(pool))
}

func provideQueryDecorator(metrics *repositorymetrics.Metrics) func(postgres.Querier) postgres.Querier {
	return metrics.Wrap
}

func provideArticleRepository(
	pool *pgxpool.Pool,
	queries postgres.Querier,
	decorate func(postgres.Querier) postgres.Querier,
) *article.PostgresRepository {
	return article.NewPostgresRepository(pool, queries, decorate)
}

func provideArticleService(
	repository *article.PostgresRepository,
	users *user.Service,
	tags *tag.Service,
	articleTags *articletag.Service,
	favorites *favorite.Service,
	follows *follow.Service,
) *article.Service {
	return article.New(repository, users, tags, articleTags, favorites, follows)
}

func provideAuthService(queries postgres.Querier, cfg *config.Config) *auth.Service {
	return auth.New(queries, cfg.Auth)
}

func provideFollowService(queries postgres.Querier) *follow.Service {
	return follow.New(queries)
}

func provideFavoriteService(queries postgres.Querier) *favorite.Service {
	return favorite.New(queries)
}

func provideArticleTagService(queries postgres.Querier) *articletag.Service {
	return articletag.New(queries)
}

func provideUserService(queries postgres.Querier, follows *follow.Service, cfg *config.Config) *user.Service {
	return user.New(queries, follows, auth.NewPasswordManager(cfg.Auth.PasswordPepper))
}

func provideCommentService(queries postgres.Querier, users *user.Service) *comment.Service {
	return comment.New(queries, users)
}

func provideTagService(queries postgres.Querier) *tag.Service {
	return tag.New(queries)
}
