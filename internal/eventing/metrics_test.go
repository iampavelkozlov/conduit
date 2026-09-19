package eventing

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestMetricsObserve(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewMetrics(registry)
	require.NoError(t, err)
	metrics.Observe("relay_publish", ResultSuccess, time.Second)
	require.InDelta(t, 1, testutil.ToFloat64(metrics.operations.WithLabelValues("relay_publish", ResultSuccess)), 0)
	var nilMetrics *Metrics
	nilMetrics.Observe("relay_publish", ResultSuccess, time.Second)

	_, err = NewMetrics(registry)
	require.Error(t, err)
	observerOrNoop(nil).Observe("operation", ResultFailure, time.Second)
	noopObserver{}.Observe("operation", ResultFailure, time.Second)
}

type failingRegisterer struct{}

func (failingRegisterer) Register(prometheus.Collector) error  { return errors.New("register") }
func (failingRegisterer) MustRegister(...prometheus.Collector) {}
func (failingRegisterer) Unregister(prometheus.Collector) bool { return false }

func TestMetricsRegistrationFailure(t *testing.T) {
	_, err := NewMetrics(failingRegisterer{})
	require.Error(t, err)
}
