// Package logging provides structured logging with zerolog.
package logging

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
)

type contextKey string

const loggerKey contextKey = "logger"

// Logger wraps zerolog.Logger with service context.
type Logger struct {
	zerolog.Logger
	serviceName string
}

// NewLogger creates a new structured logger for the given service.
func NewLogger(serviceName string, debug bool) *Logger {
	zerolog.TimeFieldFormat = time.RFC3339

	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}

	logger := zerolog.New(os.Stdout).
		Level(level).
		With().
		Timestamp().
		Str("service", serviceName).
		Logger()

	return &Logger{
		Logger:      logger,
		serviceName: serviceName,
	}
}

// WithContext returns a new logger with values from the context added.
func (l *Logger) WithContext(ctx context.Context) zerolog.Logger {
	logger := l.Logger

	// Add trace_id if present
	if traceID := ctx.Value("trace_id"); traceID != nil {
		if id, ok := traceID.(string); ok {
			logger = logger.With().Str("trace_id", id).Logger()
		}
	}

	// Add request_id if present
	if requestID := ctx.Value("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			logger = logger.With().Str("request_id", id).Logger()
		}
	}

	// Add user_id if present
	if userID := ctx.Value("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			logger = logger.With().Str("user_id", id).Logger()
		}
	}

	return logger
}

// Middleware returns chi-compatible middleware for request logging.
func (l *Logger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create logger with request context
		logger := l.WithContext(r.Context())

		// Log incoming request
		logger.Debug().
			Str("http.method", r.Method).
			Str("http.path", r.URL.Path).
			Str("http.remote_addr", r.RemoteAddr).
			Msg("request started")

		// Wrap response writer to capture status
		ww := &responseWriter{ResponseWriter: w}

		// Add logger to request context
		ctx := context.WithValue(r.Context(), loggerKey, &logger)

		// Process request
		next.ServeHTTP(ww, r.WithContext(ctx))

		// Log completed request
		duration := time.Since(start)
		logEvent := logger.Info()

		// Use Error level for 5xx status codes
		if ww.statusCode >= 500 {
			logEvent = logger.Error()
		} else if ww.statusCode >= 400 {
			logEvent = logger.Warn()
		}

		logEvent.
			Str("http.method", r.Method).
			Str("http.path", r.URL.Path).
			Int("http.status", ww.statusCode).
			Int64("duration_ms", duration.Milliseconds()).
			Msg("request completed")
	})
}

// FromContext extracts the logger from the request context.
func FromContext(ctx context.Context) *zerolog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*zerolog.Logger); ok {
		return logger
	}
	// Return a default logger if not found
	defaultLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	return &defaultLogger
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
