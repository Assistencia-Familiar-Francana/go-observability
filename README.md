# go-observability

Shared observability package for Funeraria Francana microservices implementing ADR-005 requirements.

## Features

- **Prometheus Metrics**: HTTP request instrumentation (rate, latency, in-flight requests)
- **Structured Logging**: Zerolog-based JSON logging with trace context
- **Health Checks**: Liveness (`/healthz`) and readiness (`/readyz`) endpoints
- **Trace Propagation**: X-Request-ID and X-Trace-ID header management
- **Chi Integration**: Drop-in middleware for chi routers

## Installation

```bash
go get github.com/Assistencia-Familiar-Francana/go-observability
```

## Quick Start

```go
package main

import (
    "net/http"
    obs "github.com/Assistencia-Familiar-Francana/go-observability"
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
)

func main() {
    // Initialize logger
    logger := obs.NewLogger("my-service", false) // service name, debug mode

    // Setup router with observability stack
    r := chi.NewRouter()

    // Middleware order matters!
    r.Use(middleware.RequestID)           // Generate request ID
    r.Use(obs.TraceMiddleware())          // Propagate trace context
    r.Use(obs.MetricsMiddleware("my_service"))  // Collect metrics
    r.Use(obs.LoggingMiddleware(logger))  // Log requests
    r.Use(middleware.Recoverer)

    // Health endpoints
    r.Get("/healthz", obs.LivenessHandler())
    r.Get("/readyz", obs.ReadinessHandler(
        obs.DatabaseChecker(db),
        obs.RedisChecker(redisClient),
    ))

    // Metrics endpoint
    r.Handle("/metrics", obs.MetricsHandler())

    // Your API routes
    r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
        logger := obs.LoggerFromContext(r.Context())
        logger.Info().Msg("Hello endpoint called")
        w.Write([]byte("Hello World"))
    })

    http.ListenAndServe(":8080", r)
}
```

## Metrics Exposed

All services automatically expose:

| Metric | Type | Description |
|--------|------|-------------|
| `<service>_http_requests_total` | Counter | Total HTTP requests by method/path/status |
| `<service>_http_request_duration_seconds` | Histogram | Request latency distribution |
| `<service>_http_requests_in_flight` | Gauge | Current number of requests being processed |
| `<service>_errors_total` | Counter | Total errors by type |

## Log Fields

All logs include:

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | ISO8601 | Event time |
| `level` | string | Log level (debug, info, warn, error) |
| `message` | string | Human-readable message |
| `service` | string | Service name |
| `trace_id` | string | Distributed trace ID |
| `request_id` | string | Unique request identifier |
| `http.method` | string | HTTP method (when applicable) |
| `http.path` | string | Request path (when applicable) |
| `http.status` | int | Response status code (when applicable) |
| `duration_ms` | int | Operation duration (when applicable) |

## Health Checks

### Liveness Probe (`/healthz`)
Returns 200 OK if the process is running. Used by Kubernetes to restart crashed pods.

### Readiness Probe (`/readyz`)
Returns 200 OK if the service can handle requests. Checks:
- Database connectivity
- Redis connectivity
- Custom dependency checks

Example with custom checker:

```go
customChecker := func(ctx context.Context) error {
    // Check your dependency
    if !myDependency.IsHealthy() {
        return errors.New("dependency unhealthy")
    }
    return nil
}

r.Get("/readyz", obs.ReadinessHandler(
    obs.DatabaseChecker(db),
    obs.CustomChecker("my-dep", customChecker),
))
```

## Trace Context

The middleware automatically:
1. Extracts `X-Request-ID` from incoming requests (or generates one)
2. Extracts `X-Trace-ID` from incoming requests (or generates one)
3. Adds trace IDs to request context
4. Includes trace IDs in response headers
5. Logs all requests with trace context

Access trace IDs in your handlers:

```go
func MyHandler(w http.ResponseWriter, r *http.Request) {
    traceID := obs.TraceIDFromContext(r.Context())
    requestID := obs.RequestIDFromContext(r.Context())

    logger := obs.LoggerFromContext(r.Context())
    logger.Info().
        Str("trace_id", traceID).
        Str("request_id", requestID).
        Msg("Processing request")
}
```

## ADR-005 Compliance

This package implements all requirements from ADR-005:
- ✅ Prometheus metrics endpoint
- ✅ Structured JSON logging
- ✅ Health and readiness endpoints
- ✅ Trace context propagation
- ✅ Request/response logging

## Architecture

```
go-observability/
├── metrics/         # Prometheus instrumentation
│   └── prometheus.go
├── logging/         # Zerolog configuration
│   └── logger.go
├── health/          # Health check endpoints
│   └── health.go
├── trace/           # Trace context management
│   └── trace.go
└── middleware/      # Chi HTTP middleware
    └── middleware.go
```

## Development

```bash
# Run tests
go test ./...

# Update dependencies
go mod tidy

# Vendor dependencies
go mod vendor
```

## Integration with Kubernetes

### ServiceMonitor (Prometheus)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: my-service
  labels:
    app.kubernetes.io/name: my-service
    app.kubernetes.io/part-of: funeraria-francana
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: my-service
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

### Deployment Probes

```yaml
spec:
  containers:
  - name: my-service
    livenessProbe:
      httpGet:
        path: /healthz
        port: 8080
      initialDelaySeconds: 10
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /readyz
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
```

## References

- ADR-005: Mandatory Observability
- [Prometheus Go Client](https://github.com/prometheus/client_golang)
- [Zerolog](https://github.com/rs/zerolog)
- [Chi Router](https://github.com/go-chi/chi)
