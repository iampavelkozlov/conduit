package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRegistryIncludesRuntimeCollectors(t *testing.T) {
	metricFamilies, err := NewRegistry().Gather()
	require.NoError(t, err)

	names := make(map[string]struct{}, len(metricFamilies))
	for _, family := range metricFamilies {
		names[family.GetName()] = struct{}{}
	}
	require.Contains(t, names, "go_goroutines")
	require.Contains(t, names, "process_cpu_seconds_total")
}

func TestScrapeHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	ScrapeHandler(NewRegistry()).ServeHTTP(
		recorder,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, metricsPath, nil),
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "go_goroutines")
}
