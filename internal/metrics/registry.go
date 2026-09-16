package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is the application's isolated Prometheus registry.
type Registry = prometheus.Registry

// NewRegistry creates a registry with Go runtime and process collectors.
func NewRegistry() *Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return registry
}

// ScrapeHandler exposes all collectors registered in registry.
func ScrapeHandler(registry *Registry) http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
