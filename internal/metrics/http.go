package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

const metricsPath = "/metrics"

var (
	httpDurationBuckets = []float64{
		0.001, 0.0025, 0.005, 0.01, 0.025, 0.05,
		0.1, 0.25, 0.5, 1, 2.5, 5, 10,
	}
	httpResponseSizeBuckets = []float64{
		128, 256, 512, 1024, 2048, 4096, 8192,
		16384, 32768, 65536, 131072, 262144, 524288, 1048576,
	}
)

// PanicReporter receives recovered panic details after the panic metric is
// incremented. The route is a bounded-cardinality Chi template.
// Implementations must not panic.
type PanicReporter func(r *http.Request, route string, recovered any, stack []byte)

// HTTP owns bounded-cardinality metrics for the HTTP server.
type HTTP struct {
	requests     *prometheus.CounterVec
	duration     *prometheus.HistogramVec
	responseSize *prometheus.HistogramVec
	inFlight     prometheus.Gauge
	panics       *prometheus.CounterVec
}

// NewHTTP registers HTTP metrics in the supplied application registry.
func NewHTTP(registerer prometheus.Registerer) (*HTTP, error) {
	metrics := &HTTP{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "conduit",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests partitioned by method, route, and response status.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "conduit",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   httpDurationBuckets,
		}, []string{"method", "route", "status"}),
		responseSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "conduit",
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes.",
			Buckets:   httpResponseSizeBuckets,
		}, []string{"method", "route", "status"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "conduit",
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Number of HTTP requests currently being handled.",
		}),
		panics: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "conduit",
			Subsystem: "http",
			Name:      "panics_total",
			Help:      "Total recovered HTTP handler panics partitioned by method and route.",
		}, []string{"method", "route"}),
	}

	collectors := []prometheus.Collector{
		metrics.requests,
		metrics.duration,
		metrics.responseSize,
		metrics.inFlight,
		metrics.panics,
	}
	for i, collector := range collectors {
		if err := registerer.Register(collector); err != nil {
			for _, registered := range collectors[:i] {
				registerer.Unregister(registered)
			}
			return nil, fmt.Errorf("register HTTP metric: %w", err)
		}
	}

	return metrics, nil
}

// Handler records standard HTTP request, latency, response size, and in-flight
// metrics. The Prometheus scrape endpoint is excluded to avoid self-generated
// traffic. Route templates are used instead of raw paths to bound cardinality.
func (m *HTTP) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == metricsPath {
			next.ServeHTTP(w, r)
			return
		}

		started := time.Now()
		responseWriter := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		m.inFlight.Inc()
		defer func() {
			m.inFlight.Dec()
			status := responseWriter.Status()
			if status == 0 {
				status = http.StatusOK
			}
			labels := []string{metricMethod(r.Method), routePattern(r), strconv.Itoa(status)}
			m.requests.WithLabelValues(labels...).Inc()
			m.duration.WithLabelValues(labels...).Observe(time.Since(started).Seconds())
			m.responseSize.WithLabelValues(labels...).Observe(float64(responseWriter.BytesWritten()))
		}()

		next.ServeHTTP(responseWriter, r)
	})
}

// Recoverer converts handler panics into the canonical internal-error response,
// reports the stack, and increments the panic counter. Register it after Handler
// so recovered panics are also observed as HTTP 500 responses.
func (m *HTTP) Recoverer(reporter PanicReporter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				route := routePattern(r)
				m.panics.WithLabelValues(metricMethod(r.Method), route).Inc()
				if reporter != nil {
					reporter(r, route, recovered, debug.Stack())
				}
				writeInternalServerError(w)
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func routePattern(r *http.Request) string {
	pattern := chi.RouteContext(r.Context()).RoutePattern()
	if pattern == "" {
		return "unmatched"
	}
	return pattern
}

func metricMethod(method string) string {
	switch method {
	case http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead,
		http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace:
		return method
	default:
		return "OTHER"
	}
}

func writeInternalServerError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]map[string][]string{
		"errors": {"body": {"internal server error"}},
	})
}
