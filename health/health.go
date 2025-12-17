// Package health provides health check endpoints for Kubernetes probes.
package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// Checker defines a health check function.
type Checker func(ctx context.Context) error

// CheckResult represents the result of a health check.
type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok" or "error"
	Error  string `json:"error,omitempty"`
}

// HealthResponse represents the full health check response.
type HealthResponse struct {
	Status string        `json:"status"` // "ok" or "error"
	Checks []CheckResult `json:"checks,omitempty"`
}

// LivenessHandler returns a simple liveness probe handler.
// This should always return 200 OK if the process is running.
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// ReadinessHandler returns a readiness probe handler that checks dependencies.
// Returns 200 OK only if all checks pass, 503 Service Unavailable otherwise.
func ReadinessHandler(checkers ...Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		results := make([]CheckResult, 0, len(checkers))
		allOK := true

		for _, checker := range checkers {
			// Extract checker name from context if available
			name := "dependency"
			if nameCtx, ok := checker.(interface{ Name() string }); ok {
				name = nameCtx.Name()
			}

			result := CheckResult{Name: name, Status: "ok"}
			if err := checker(ctx); err != nil {
				result.Status = "error"
				result.Error = err.Error()
				allOK = false
			}
			results = append(results, result)
		}

		response := HealthResponse{
			Status: "ok",
			Checks: results,
		}

		statusCode := http.StatusOK
		if !allOK {
			response.Status = "error"
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}

// namedChecker wraps a checker with a name.
type namedChecker struct {
	name    string
	checker Checker
}

func (n *namedChecker) Name() string {
	return n.name
}

func (n *namedChecker) Call(ctx context.Context) error {
	return n.checker(ctx)
}

// DatabaseChecker returns a health checker for database connectivity.
func DatabaseChecker(db *sql.DB) Checker {
	checker := &namedChecker{
		name: "database",
		checker: func(ctx context.Context) error {
			return db.PingContext(ctx)
		},
	}
	return checker.Call
}

// CustomChecker creates a named health checker from a function.
func CustomChecker(name string, checker Checker) Checker {
	nc := &namedChecker{
		name:    name,
		checker: checker,
	}
	return nc.Call
}

// RedisChecker returns a health checker for Redis connectivity.
// Accepts any client that implements a Ping() method.
func RedisChecker(client interface{ Ping(ctx context.Context) error }) Checker {
	checker := &namedChecker{
		name: "redis",
		checker: func(ctx context.Context) error {
			return client.Ping(ctx)
		},
	}
	return checker.Call
}

// HTTPChecker returns a health checker that performs an HTTP GET request.
func HTTPChecker(name, url string) Checker {
	checker := &namedChecker{
		name: name,
		checker: func(ctx context.Context) error {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return err
			}

			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				return http.ErrAbortHandler
			}
			return nil
		},
	}
	return checker.Call
}
