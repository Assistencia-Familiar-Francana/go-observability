// Package metrics provides Prometheus instrumentation for HTTP services.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Collector holds Prometheus metrics for a service.
type Collector struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestsInFlight prometheus.Gauge
	errorsTotal      *prometheus.CounterVec
}

// NewCollector creates a new metrics collector for the given service namespace.
func NewCollector(namespace string) *Collector {
	return &Collector{
		requestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		requestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "http_requests_in_flight",
				Help:      "Current number of HTTP requests being processed",
			},
		),
		errorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "errors_total",
				Help:      "Total number of errors by type",
			},
			[]string{"type"},
		),
	}
}

// Middleware returns chi-compatible middleware for HTTP metrics collection.
func (c *Collector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip metrics endpoint itself to avoid recursion
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		c.requestsInFlight.Inc()

		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		defer func() {
			c.requestsInFlight.Dec()

			duration := time.Since(start).Seconds()

			// Use the route pattern if available (chi-specific), otherwise use path
			path := r.URL.Path
			if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
				path = rctx.RoutePattern()
			}

			c.requestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(ww.statusCode)).Inc()
			c.requestDuration.WithLabelValues(r.Method, path).Observe(duration)
		}()

		next.ServeHTTP(ww, r)
	})
}

// RecordError increments the error counter for the given error type.
func (c *Collector) RecordError(errorType string) {
	c.errorsTotal.WithLabelValues(errorType).Inc()
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code and delegates to the underlying ResponseWriter.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Write ensures WriteHeader is called with 200 OK if not explicitly set.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter (for middleware compatibility).
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}
