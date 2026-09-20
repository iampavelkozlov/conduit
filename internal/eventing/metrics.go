package eventing

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	ResultSuccess = "success"
	ResultFailure = "failure"
)

type Observer interface {
	Observe(string, string, time.Duration)
}

type Metrics struct {
	operations *prometheus.CounterVec
	duration   *prometheus.HistogramVec
}

func NewMetrics(registerer prometheus.Registerer) (*Metrics, error) {
	metrics := &Metrics{
		operations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "conduit", Subsystem: "eventing", Name: "operations_total",
			Help: "Eventing operations by bounded operation and result.",
		}, []string{"operation", "result"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "conduit", Subsystem: "eventing", Name: "operation_duration_seconds",
			Help: "Eventing operation duration by bounded operation.",
		}, []string{"operation"}),
	}
	for _, collector := range []prometheus.Collector{metrics.operations, metrics.duration} {
		if err := registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("register eventing metrics: %w", err)
		}
	}
	return metrics, nil
}

func (m *Metrics) Observe(operation, result string, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.operations.WithLabelValues(operation, result).Inc()
	m.duration.WithLabelValues(operation).Observe(elapsed.Seconds())
}

type noopObserver struct{}

func (noopObserver) Observe(string, string, time.Duration) {}

func observerOrNoop(observer Observer) Observer {
	if observer == nil {
		return noopObserver{}
	}
	return observer
}
