package health

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Check is a health probe that carries the identity it is reported under.
type Check interface {
	// Name identifies the dependency in a CheckResult.
	Name() string
	// Call returns nil when the dependency is usable.
	Call(ctx context.Context) error
}

// Checker is a bare probe function. Named adapts one into a Check.
type Checker func(ctx context.Context) error

type namedCheck struct {
	name  string
	probe Checker
}

func (n namedCheck) Name() string                   { return n.name }
func (n namedCheck) Call(ctx context.Context) error { return n.probe(ctx) }

// Named pairs a bare probe with the identity it is reported under.
func Named(name string, probe Checker) Check {
	return namedCheck{name: name, probe: probe}
}

// CustomChecker pairs a bare probe with the identity it is reported under.
func CustomChecker(name string, probe Checker) Check {
	return Named(name, probe)
}

// DatabaseChecker reports whether db answers a ping. A nil handle is reported
// as an unhealthy dependency, never as a panic.
func DatabaseChecker(db *sql.DB) Check {
	return Named("database", func(ctx context.Context) error {
		if db == nil {
			return errors.New("no database handle")
		}
		return db.PingContext(ctx)
	})
}

// Pinger is any client answering a context-aware ping.
type Pinger interface {
	Ping(ctx context.Context) error
}

// RedisChecker reports whether client answers a ping. A nil client is reported
// as an unhealthy dependency, never as a panic.
func RedisChecker(client Pinger) Check {
	return Named("redis", func(ctx context.Context) error {
		if client == nil {
			return errors.New("no redis client")
		}
		return client.Ping(ctx)
	})
}

// UnexpectedStatusError reports an HTTP probe answered at or above 400.
type UnexpectedStatusError struct {
	URL        string
	StatusCode int
}

func (e *UnexpectedStatusError) Error() string {
	return fmt.Sprintf("GET %s: unexpected status %d", e.URL, e.StatusCode)
}

// HTTPChecker reports whether a GET on url answers below 400.
func HTTPChecker(name, url string) Check {
	client := &http.Client{Timeout: 3 * time.Second}
	return Named(name, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return &UnexpectedStatusError{URL: url, StatusCode: resp.StatusCode}
		}
		return nil
	})
}
