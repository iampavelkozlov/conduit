package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"conduit/internal/config"
	api "conduit/internal/gen/http"
	"conduit/internal/metrics"
	"conduit/internal/service"
	httptransport "conduit/internal/transport/http"
	transportmiddleware "conduit/internal/transport/middleware"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

const maxRequestBodySize = 1 << 20

func provideHTTPHandler(
	cfg *config.Config,
	authMiddleware *transportmiddleware.AuthMiddleware,
	httpMetrics *metrics.HTTP,
	panicReporter metrics.PanicReporter,
	server api.StrictServerInterface,
	svc *service.Service,
	registry *metrics.Registry,
) (http.Handler, error) {
	spec, err := api.GetSwagger()
	if err != nil {
		return nil, err
	}
	// The canonical specification advertises the production host. Runtime
	// validation remains host-agnostic without changing the immutable schema.
	spec.Servers = openapi3.Servers{new(openapi3.Server{URL: "/api"})}

	strictHandler := api.NewStrictHandlerWithOptions(server, nil, api.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: httptransport.NewResponseErrorHandler(),
	})

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(httpMetrics.Handler)
	router.Use(httpMetrics.Recoverer(panicReporter))
	router.Handle("/metrics", metrics.ScrapeHandler(registry))
	router.Method(http.MethodPost, "/internal/auth/refresh", httptransport.NewRefreshHandler(svc))
	router.Group(func(apiRouter chi.Router) {
		apiRouter.Use(chimiddleware.RequestSize(maxRequestBodySize))
		apiRouter.Use(transportmiddleware.JSONHeaders)
		apiRouter.Use(authMiddleware.OptionalAuth)
		apiRouter.Use(nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
			Options:               openapi3filter.Options{AuthenticationFunc: authMiddleware.Authenticate},
			SilenceServersWarning: true,
			ErrorHandler: func(w http.ResponseWriter, message string, status int) {
				if status == http.StatusUnauthorized {
					value := "invalid"
					if strings.Contains(message, "missing Authorization") {
						value = "is missing"
					}
					writeError(w, http.StatusUnauthorized, "token", value)
					return
				}
				if status == http.StatusBadRequest {
					status = http.StatusUnprocessableEntity
				}
				writeError(w, status, "body", "invalid request")
			},
		}))
		apiRouter.Use(chimiddleware.AllowContentType("application/json"))
		apiRouter.Use(chimiddleware.CleanPath)
		apiRouter.Use(cors.Handler(cors.Options{
			AllowedOrigins: cfg.HTTP.AllowedOrigins,
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders: []string{"Link"},
			MaxAge:         300,
		}))
		apiRouter.Use(chimiddleware.StripSlashes)
		apiRouter.Use(httptransport.TrackUpdateFields)
		api.HandlerFromMuxWithBaseURL(strictHandler, apiRouter, "/api")
	})
	return router, nil
}

func writeError(w http.ResponseWriter, status int, field, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.GenericErrorModel{Errors: map[string][]string{field: {message}}})
}
