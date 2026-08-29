package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// DefaultTimeout bounds one readiness evaluation.
const DefaultTimeout = 5 * time.Second

// HTTPStatus is the response code a Status is served with.
func HTTPStatus(status Status) int {
	switch status {
	case StatusOK:
		return http.StatusOK
	case StatusError:
		return http.StatusServiceUnavailable
	}
	// An unnamed Status is not a healthy one.
	return http.StatusServiceUnavailable
}

// LivenessHandler answers 200 for as long as the process is running.
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, HealthResponse{Status: StatusOK})
	}
}

// ReadinessHandler answers 200 only when every check passes, 503 otherwise.
func ReadinessHandler(checks ...Check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), DefaultTimeout)
		defer cancel()

		response := Summarize(Evaluate(ctx, checks))
		writeJSON(w, HTTPStatus(response.Status), response)
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
