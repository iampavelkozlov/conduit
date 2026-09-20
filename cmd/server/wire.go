//go:build wireinject

package main

import (
	"context"

	"conduit/internal/config"
	"conduit/internal/transport/http"

	"github.com/google/wire"
)

//go:generate go tool wire
func initializeApplication(ctx context.Context, configPath string) (*application, func(), error) {
	wire.Build(
		config.Load,
		provideLogger,
		providePrometheusRegistry,
		provideHTTPMetrics,
		providePanicReporter,
		provideApplicationService,
		provideAuthMiddleware,
		http.NewServer,
		provideHTTPHandler,
		newApplication,
	)
	return nil, nil, nil
}
