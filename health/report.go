// Package health provides health check endpoints for Kubernetes probes.
package health

// Status is the outcome of a health evaluation. The set is closed.
type Status string

const (
	// StatusOK reports a usable dependency, or a wholly usable service.
	StatusOK Status = "ok"
	// StatusError reports an unusable dependency, or a service with at least one.
	StatusError Status = "error"
)

// CheckResult is the outcome of one named probe.
type CheckResult struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
}

// HealthResponse is the body served by the liveness and readiness handlers.
type HealthResponse struct {
	Status Status        `json:"status"`
	Checks []CheckResult `json:"checks,omitempty"`
}

// Summarize folds per-check results into one response: StatusOK only when
// every result is StatusOK. Pure.
func Summarize(results []CheckResult) HealthResponse {
	status := StatusOK
	for _, result := range results {
		if result.Status != StatusOK {
			status = StatusError
			break
		}
	}
	return HealthResponse{Status: status, Checks: results}
}
