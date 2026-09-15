package repositorymetrics

import (
	"fmt"
	"time"

	"conduit/internal/gen/postgres"

	"github.com/prometheus/client_golang/prometheus"
)

//go:generate go tool gowrap gen -g -p conduit/internal/gen/postgres -i Querier -t ../../../wrap/prometheus.tpl -o querier_metrics_gen.go -l conduit

var repositoryDurationBuckets = []float64{
	0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05,
	0.1, 0.25, 0.5, 1, 2.5, 5,
}

// Metrics owns bounded-cardinality metrics for generated repository methods.
type Metrics struct {
	calls    *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// New registers repository metrics in the supplied application registry.
func New(registerer prometheus.Registerer) (*Metrics, error) {
	calls := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "conduit",
		Subsystem: "repository",
		Name:      "method_calls_total",
		Help:      "Total repository method calls partitioned by method and result.",
	}, []string{"method", "result"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "conduit",
		Subsystem: "repository",
		Name:      "method_duration_seconds",
		Help:      "Repository method execution duration in seconds.",
		Buckets:   repositoryDurationBuckets,
	}, []string{"method"})

	if err := registerer.Register(calls); err != nil {
		return nil, fmt.Errorf("register repository calls metric: %w", err)
	}
	if err := registerer.Register(duration); err != nil {
		registerer.Unregister(calls)
		return nil, fmt.Errorf("register repository duration metric: %w", err)
	}
	return &Metrics{calls: calls, duration: duration}, nil
}

// Wrap decorates any sqlc Querier, including transaction-bound query sets.
func (m *Metrics) Wrap(querier postgres.Querier) postgres.Querier {
	return NewQuerierMetrics(querier, m)
}

func (m *Metrics) observe(method string, started time.Time, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	m.calls.WithLabelValues(method, result).Inc()
	m.duration.WithLabelValues(method).Observe(time.Since(started).Seconds())
}
