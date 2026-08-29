package health

import (
	"context"
	"fmt"
	"sync"
)

// Evaluate runs every check and returns one result per check, in the order the
// checks were given. Checks run concurrently, so ctx's deadline bounds the
// slowest check rather than their sum. A check that panics yields an error
// result; it never propagates.
func Evaluate(ctx context.Context, checks []Check) []CheckResult {
	results := make([]CheckResult, len(checks))
	var wg sync.WaitGroup
	for i, check := range checks {
		wg.Add(1)
		go func(i int, check Check) {
			defer wg.Done()
			results[i] = runCheck(ctx, check)
		}(i, check)
	}
	wg.Wait()
	return results
}

func runCheck(ctx context.Context, check Check) (result CheckResult) {
	result = CheckResult{Name: "dependency", Status: StatusOK}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.Status = StatusError
			result.Error = fmt.Sprintf("panic: %v", recovered)
		}
	}()

	if check == nil {
		result.Status = StatusError
		result.Error = "nil check"
		return result
	}

	result.Name = check.Name()
	if err := check.Call(ctx); err != nil {
		result.Status = StatusError
		result.Error = err.Error()
	}
	return result
}
