// Package observability provides a unified observability stack for Go microservices.
// It implements ADR-005 requirements including metrics, logging, health checks, and tracing.
package observability

import (
	"context"
	"net/http"

	"github.com/Assistencia-Familiar-Francana/go-observability/health"
	"github.com/Assistencia-Familiar-Francana/go-observability/logging"
	"github.com/Assistencia-Familiar-Francana/go-observability/metrics"
	"github.com/Assistencia-Familiar-Francana/go-observability/trace"
	"github.com/rs/zerolog"
)

// Stack holds all observability components for a service.
type Stack struct {
	logger    *logging.Logger
	collector *metrics.Collector
}

// NewStack creates a new observability stack for the given service.
func NewStack(serviceName string, debug bool) *Stack {
	return &Stack{
		logger:    logging.NewLogger(serviceName, debug),
		collector: metrics.NewCollector(serviceName),
	}
}

// Logger returns the structured logger.
func (s *Stack) Logger() *logging.Logger {
	return s.logger
}

// Collector returns the metrics collector.
func (s *Stack) Collector() *metrics.Collector {
	return s.collector
}

// TraceMiddleware returns middleware for trace context propagation.
func TraceMiddleware() func(http.Handler) http.Handler {
	return trace.Middleware
}

// MetricsMiddleware returns middleware for metrics collection.
func (s *Stack) MetricsMiddleware() func(http.Handler) http.Handler {
	return s.collector.Middleware
}

// LoggingMiddleware returns middleware for request logging.
func (s *Stack) LoggingMiddleware() func(http.Handler) http.Handler {
	return s.logger.Middleware
}

// MetricsHandler returns the Prometheus metrics HTTP handler.
func MetricsHandler() http.Handler {
	return metrics.Handler()
}

// LivenessHandler returns a liveness probe handler.
func LivenessHandler() http.HandlerFunc {
	return health.LivenessHandler()
}

// ReadinessHandler returns a readiness probe handler with dependency checks.
func ReadinessHandler(checkers ...health.Checker) http.HandlerFunc {
	return health.ReadinessHandler(checkers...)
}

// Convenient re-exports for common types
type (
	Logger  = logging.Logger
	Checker = health.Checker
)

// Health check constructors
var (
	DatabaseChecker = health.DatabaseChecker
	CustomChecker   = health.CustomChecker
	RedisChecker    = health.RedisChecker
	HTTPChecker     = health.HTTPChecker
)

// Trace context extractors
var (
	TraceIDFromContext   = trace.TraceIDFromContext
	RequestIDFromContext = trace.RequestIDFromContext
)

// Logger context extractor
func LoggerFromContext(ctx context.Context) *zerolog.Logger {
	return logging.FromContext(ctx)
}
