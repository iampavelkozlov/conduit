package repositorymetrics

import (
	"context"
	"errors"
	"testing"

	"conduit/internal/gen/postgres"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestMetricsObserve(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := New(registry)
	require.NoError(t, err)

	_, err = metrics.Wrap(querierStub{}).ListTags(t.Context())
	require.NoError(t, err)
	_, err = metrics.Wrap(querierStub{err: errors.New("query failed")}).ListTags(t.Context())
	require.Error(t, err)

	require.InDelta(t, 1, testutil.ToFloat64(metrics.calls.WithLabelValues("ListTags", "success")), 0)
	require.InDelta(t, 1, testutil.ToFloat64(metrics.calls.WithLabelValues("ListTags", "error")), 0)
	require.Equal(t, 1, testutil.CollectAndCount(metrics.duration, "conduit_repository_method_duration_seconds"))
}

type querierStub struct {
	postgres.Querier
	err error
}

func (s querierStub) ListTags(context.Context) ([]string, error) {
	return []string{"go"}, s.err
}

func TestMetricsRejectDuplicateRegistration(t *testing.T) {
	registry := prometheus.NewRegistry()
	_, err := New(registry)
	require.NoError(t, err)

	_, err = New(registry)
	require.Error(t, err)
}

func TestMetricsUnregistersCallsWhenDurationRegistrationFails(t *testing.T) {
	registerer := &failingRegisterer{failAt: 2, err: errors.New("registration failed")}

	metrics, err := New(registerer)

	require.ErrorIs(t, err, registerer.err)
	require.Nil(t, metrics)
	require.Equal(t, 1, registerer.unregisterCalls)
}

type failingRegisterer struct {
	calls           int
	failAt          int
	err             error
	unregisterCalls int
}

func (r *failingRegisterer) Register(prometheus.Collector) error {
	r.calls++
	if r.calls == r.failAt {
		return r.err
	}
	return nil
}

func (r *failingRegisterer) MustRegister(...prometheus.Collector) {}

func (r *failingRegisterer) Unregister(prometheus.Collector) bool {
	r.unregisterCalls++
	return true
}
