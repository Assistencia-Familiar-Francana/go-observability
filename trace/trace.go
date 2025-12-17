// Package trace provides trace context propagation for distributed tracing.
package trace

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const (
	traceIDKey   contextKey = "trace_id"
	requestIDKey contextKey = "request_id"
)

// Middleware returns chi-compatible middleware for trace context propagation.
// It extracts or generates X-Trace-ID and X-Request-ID headers and adds them to the request context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract or generate trace ID
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.NewString()
		}

		// Extract or generate request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		// Add to context
		ctx := context.WithValue(r.Context(), traceIDKey, traceID)
		ctx = context.WithValue(ctx, requestIDKey, requestID)

		// Add to response headers for client correlation
		w.Header().Set("X-Trace-ID", traceID)
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceIDFromContext extracts the trace ID from the context.
func TraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// RequestIDFromContext extracts the request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// WithTraceID returns a new context with the trace ID set.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// WithRequestID returns a new context with the request ID set.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}
