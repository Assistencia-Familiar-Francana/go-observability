package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Assistencia-Familiar-Francana/go-observability/health"
)

func decode(t *testing.T, body string) health.HealthResponse {
	t.Helper()
	var response health.HealthResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("body is not a HealthResponse: %v (%q)", err, body)
	}
	return response
}

func readiness(t *testing.T, checks ...health.Check) (int, health.HealthResponse) {
	t.Helper()
	recorder := httptest.NewRecorder()
	health.ReadinessHandler(checks...).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	return recorder.Code, decode(t, recorder.Body.String())
}

// A constructor pairs a probe with a name, and the name must survive all the
// way into the response. Before this test the constructors built a name and
// then returned a bare func, discarding it.
func TestConstructorsReportTheirOwnName(t *testing.T) {
	cases := []struct {
		check health.Check
		want  string
	}{
		{health.DatabaseChecker(nil), "database"},
		{health.RedisChecker(nil), "redis"},
		{health.HTTPChecker("keycloak", "http://127.0.0.1:1/"), "keycloak"},
		{health.CustomChecker("nats", func(context.Context) error { return nil }), "nats"},
		{health.Named("firebird", func(context.Context) error { return nil }), "firebird"},
	}

	for _, tc := range cases {
		if got := tc.check.Name(); got != tc.want {
			t.Errorf("Name() = %q, want %q", got, tc.want)
		}
	}

	_, response := readiness(t, health.DatabaseChecker(nil), health.CustomChecker("nats", func(context.Context) error { return nil }))
	if len(response.Checks) != 2 {
		t.Fatalf("got %d results, want 2", len(response.Checks))
	}
	if response.Checks[0].Name != "database" || response.Checks[1].Name != "nats" {
		t.Errorf("names not carried into the response: %+v", response.Checks)
	}
}

func TestReadinessIsOKWhenEveryCheckPasses(t *testing.T) {
	ok := health.Named("ok", func(context.Context) error { return nil })

	code, response := readiness(t, ok, ok)

	if code != http.StatusOK {
		t.Errorf("status = %d, want 200", code)
	}
	if response.Status != health.StatusOK {
		t.Errorf("body status = %q, want ok", response.Status)
	}
}

func TestReadinessIs503WhenOneCheckFails(t *testing.T) {
	code, response := readiness(t,
		health.Named("ok", func(context.Context) error { return nil }),
		health.Named("broken", func(context.Context) error { return errors.New("connection refused") }),
	)

	if code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", code)
	}
	if response.Status != health.StatusError {
		t.Errorf("body status = %q, want error", response.Status)
	}
	if response.Checks[0].Status != health.StatusOK {
		t.Errorf("healthy check reported %q", response.Checks[0].Status)
	}
	if response.Checks[1].Error != "connection refused" {
		t.Errorf("failure text lost: %+v", response.Checks[1])
	}
}

// Results are indexed by position, so a concurrent evaluation must not reorder
// them relative to the checks it was given.
func TestResultsKeepTheOrderOfTheirChecks(t *testing.T) {
	slow := health.Named("slow", func(context.Context) error {
		time.Sleep(40 * time.Millisecond)
		return nil
	})
	fast := health.Named("fast", func(context.Context) error { return nil })

	results := health.Evaluate(context.Background(), []health.Check{slow, fast, slow})

	got := []string{results[0].Name, results[1].Name, results[2].Name}
	want := []string{"slow", "fast", "slow"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// The whole evaluation shares one deadline. Run sequentially, three 80ms checks
// consume 240ms of a 5s budget and starve whatever follows them; run
// concurrently they cost one 80ms.
func TestChecksShareTheDeadlineRatherThanConsumingItInSequence(t *testing.T) {
	slow := health.Named("slow", func(context.Context) error {
		time.Sleep(80 * time.Millisecond)
		return nil
	})

	start := time.Now()
	health.Evaluate(context.Background(), []health.Check{slow, slow, slow})
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Errorf("three 80ms checks took %v — they ran in sequence", elapsed)
	}
}

// A dependency whose probe panics is an unhealthy dependency, not a dead
// process. Regression anchor for the typed-nil class of panic.
func TestAPanickingCheckIsReportedAsUnhealthy(t *testing.T) {
	var nilMap map[string]string

	code, response := readiness(t, health.Named("explodes", func(context.Context) error {
		nilMap["boom"] = "x"
		return nil
	}))

	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
	if response.Checks[0].Name != "explodes" {
		t.Errorf("name lost through the recover: %+v", response.Checks[0])
	}
	if !strings.HasPrefix(response.Checks[0].Error, "panic: ") {
		t.Errorf("error = %q, want it to name the panic", response.Checks[0].Error)
	}
}

func TestNilDependenciesAreUnhealthyNotFatal(t *testing.T) {
	code, response := readiness(t, health.DatabaseChecker(nil), health.RedisChecker(nil))

	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
	for _, result := range response.Checks {
		if result.Status != health.StatusError {
			t.Errorf("%s reported %q", result.Name, result.Status)
		}
		if strings.HasPrefix(result.Error, "panic: ") {
			t.Errorf("%s panicked instead of reporting: %q", result.Name, result.Error)
		}
	}
}

// http.ErrAbortHandler is a server-side panic sentinel: net/http suppresses the
// stack when a handler panics with it. Using it as a client-side probe failure
// makes an unreachable dependency indistinguishable from a silenced crash.
func TestHTTPCheckerRejectsErrorStatusWithADescriptiveError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := health.HTTPChecker("upstream", server.URL).Call(context.Background())

	if err == nil {
		t.Fatal("a 500 must fail the check")
	}
	if errors.Is(err, http.ErrAbortHandler) {
		t.Fatal("check failed with http.ErrAbortHandler, a server panic sentinel")
	}
	var unexpected *health.UnexpectedStatusError
	if !errors.As(err, &unexpected) {
		t.Fatalf("err = %T(%v), want *health.UnexpectedStatusError", err, err)
	}
	if unexpected.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", unexpected.StatusCode)
	}
}

func TestHTTPCheckerAcceptsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := health.HTTPChecker("upstream", server.URL).Call(context.Background()); err != nil {
		t.Errorf("204 must pass the check, got %v", err)
	}
}

func TestSummarize(t *testing.T) {
	cases := []struct {
		name    string
		results []health.CheckResult
		want    health.Status
	}{
		{"no checks is ok", nil, health.StatusOK},
		{"all ok", []health.CheckResult{{Status: health.StatusOK}, {Status: health.StatusOK}}, health.StatusOK},
		{"one failure poisons the whole", []health.CheckResult{{Status: health.StatusOK}, {Status: health.StatusError}}, health.StatusError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := health.Summarize(tc.results).Status; got != tc.want {
				t.Errorf("Status = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHTTPStatus(t *testing.T) {
	if got := health.HTTPStatus(health.StatusOK); got != http.StatusOK {
		t.Errorf("ok -> %d, want 200", got)
	}
	if got := health.HTTPStatus(health.StatusError); got != http.StatusServiceUnavailable {
		t.Errorf("error -> %d, want 503", got)
	}
	// An unnamed Status must not be served as healthy.
	if got := health.HTTPStatus(health.Status("degraded")); got != http.StatusServiceUnavailable {
		t.Errorf("unknown -> %d, want 503", got)
	}
}

func TestLivenessAlwaysAnswersOK(t *testing.T) {
	recorder := httptest.NewRecorder()
	health.LivenessHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", recorder.Code)
	}
	if got := decode(t, recorder.Body.String()); got.Status != health.StatusOK {
		t.Errorf("body status = %q, want ok", got.Status)
	}
}
