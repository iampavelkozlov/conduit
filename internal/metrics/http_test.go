package metrics

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestHTTPMetricsHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		status         int
		body           string
		expectedMethod string
		expectedRoute  string
		expectedBytes  int
	}{
		{
			name:           "records successful request using route template",
			method:         http.MethodGet,
			path:           "/articles/a-user-provided-slug",
			status:         http.StatusOK,
			body:           "article",
			expectedMethod: http.MethodGet,
			expectedRoute:  "/articles/{slug}",
			expectedBytes:  len("article"),
		},
		{
			name:           "records error response",
			method:         http.MethodPost,
			path:           "/articles/example",
			status:         http.StatusUnprocessableEntity,
			body:           "invalid",
			expectedMethod: http.MethodPost,
			expectedRoute:  "/articles/{slug}",
			expectedBytes:  len("invalid"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := prometheus.NewRegistry()
			metrics, err := NewHTTP(registry)
			require.NoError(t, err)

			router := chi.NewRouter()
			router.Use(metrics.Handler)
			router.HandleFunc("/articles/{slug}", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil))

			require.Equal(t, tt.status, response.Code)
			require.InDelta(t, 1, testutil.ToFloat64(metrics.requests.WithLabelValues(
				tt.expectedMethod,
				tt.expectedRoute,
				strconv.Itoa(tt.status),
			)), 0)
			require.Equal(t, 1, testutil.CollectAndCount(metrics.duration, "conduit_http_request_duration_seconds"))
			require.Equal(t, 1, testutil.CollectAndCount(metrics.responseSize, "conduit_http_response_size_bytes"))
			require.InDelta(t, 0, testutil.ToFloat64(metrics.inFlight), 0)
			require.Equal(t, tt.expectedBytes, response.Body.Len())
		})
	}
}

func TestHTTPMetricsHandlerDefaultsStatusAndRoute(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewHTTP(registry)
	require.NoError(t, err)

	handler := metrics.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/unmatched", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.InDelta(t, 1, testutil.ToFloat64(metrics.requests.WithLabelValues(http.MethodGet, "unmatched", "200")), 0)
}

func TestMetricMethod(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		expected string
	}{
		{name: "keeps standard method", method: http.MethodPatch, expected: http.MethodPatch},
		{name: "normalizes extension method", method: "CUSTOM-METHOD", expected: "OTHER"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, metricMethod(tt.method))
		})
	}
}

func TestHTTPMetricsRecoverer(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewHTTP(registry)
	require.NoError(t, err)

	var recoveredValue any
	var recoveredStack []byte
	var recoveredRoute string
	router := chi.NewRouter()
	router.Use(metrics.Handler)
	router.Use(metrics.Recoverer(func(_ *http.Request, route string, recovered any, stack []byte) {
		recoveredRoute = route
		recoveredValue = recovered
		recoveredStack = stack
	}))
	router.Get("/panic/{id}", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic/42", nil))

	require.Equal(t, "boom", recoveredValue)
	require.NotEmpty(t, recoveredStack)
	require.Equal(t, "/panic/{id}", recoveredRoute)
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.JSONEq(t, `{"errors":{"body":["internal server error"]}}`, response.Body.String())
	require.InDelta(t, 1, testutil.ToFloat64(metrics.panics.WithLabelValues(http.MethodGet, "/panic/{id}")), 0)
	require.InDelta(t, 1, testutil.ToFloat64(metrics.requests.WithLabelValues(
		http.MethodGet,
		"/panic/{id}",
		"500",
	)), 0)
}

func TestHTTPMetricsRecovererPassesThroughAndSupportsNilReporter(t *testing.T) {
	tests := []struct {
		name  string
		panic bool
		code  int
	}{
		{name: "passes through normal response", code: http.StatusNoContent},
		{name: "recovers without reporter", panic: true, code: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics, err := NewHTTP(prometheus.NewRegistry())
			require.NoError(t, err)
			handler := metrics.Recoverer(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.panic {
					panic("boom")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil))
			require.Equal(t, tt.code, response.Code)
		})
	}
}

func TestHTTPMetricsSkipsScrapeEndpoint(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewHTTP(registry)
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Use(metrics.Handler)
	router.Get(metricsPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, metricsPath, nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, 0, testutil.CollectAndCount(metrics.requests))
}

func TestHTTPMetricsRejectDuplicateRegistration(t *testing.T) {
	registry := prometheus.NewRegistry()
	_, err := NewHTTP(registry)
	require.NoError(t, err)

	_, err = NewHTTP(registry)
	require.Error(t, err)
}
