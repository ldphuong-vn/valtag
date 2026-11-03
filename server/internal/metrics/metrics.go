// Package metrics provides opt-in Prometheus instrumentation: the standard
// Go runtime + process collectors plus an HTTP request counter keyed by
// route pattern (chi), method and status — never by raw path, so UUID-rich
// URLs don't explode label cardinality.
package metrics

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registry   *prometheus.Registry
	httpTotal  *prometheus.CounterVec
	registerMu sync.Once
)

// Register idempotently creates the registry and collectors. Call once at
// startup when METRICS_ENABLED is set; serve /metrics with Handler().
func Register() {
	registerMu.Do(func() {
		registry = prometheus.NewRegistry()
		registry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		)
		httpTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "valtag_http_requests_total",
				Help: "HTTP requests handled, by route pattern, method and status code.",
			},
			[]string{"route", "method", "status"},
		)
		registry.MustRegister(httpTotal)
	})
}

// Handler returns the Prometheus HTTP handler. Safe to call only after
// Register; returns nil otherwise.
func Handler() http.Handler {
	if registry == nil {
		return nil
	}
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// Middleware increments the request counter using the chi route pattern as
// the route label (falling back to "unmatched" for routes registered before
// the middleware or 404s).
func Middleware(next http.Handler) http.Handler {
	if httpTotal == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		httpTotal.WithLabelValues(route, r.Method, strconv.Itoa(wrapped.status)).Inc()
	})
}

// statusWriter records the status code written by inner handlers.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}
