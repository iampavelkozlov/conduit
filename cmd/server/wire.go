//go:build wireinject

package main

import (
	"context"

	"conduit/internal/config"
	"conduit/internal/service"
	"conduit/internal/transport/http"
	"conduit/internal/transport/middleware"

	"github.com/google/wire"
)

//go:generate go tool wire
func initializeApplication(ctx context.Context, configPath string) (*application, func(), error) {
	wire.Build(
		config.Load,
		provideLogger,
		provideDatabase,
		providePrometheusRegistry,
		provideRepositoryMetrics,
		provideHTTPMetrics,
		providePanicReporter,
		provideQueries,
		provideQueryDecorator,
		provideTransactions,
		provideAuthService,
		provideFollowService,
		provideFavoriteService,
		provideArticleTagService,
		provideUserService,
		provideArticleService,
		provideCommentService,
		provideTagService,
		service.New,
		middleware.NewAuthMiddleware,
		http.NewServer,
		provideHTTPHandler,
		newApplication,
	)
	return nil, nil, nil
}
