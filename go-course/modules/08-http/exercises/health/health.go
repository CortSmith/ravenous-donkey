// Package health implements a JSON health-check endpoint whose individual
// checks are bounded by a per-check deadline derived from the request context.
package health

import (
	"context"
	"net/http"
	"time"
)

// Check reports whether one dependency (database, cache, upstream API, ...)
// is healthy. A Check must respect ctx: when ctx is done it should give up
// promptly, typically by returning ctx.Err().
type Check func(ctx context.Context) error

// Response is the JSON document written by Handler.
type Response struct {
	// Status is "ok" when every check passed and "fail" otherwise.
	Status string `json:"status"`
	// Checks maps each check name to "ok" or to the check's error text.
	Checks map[string]string `json:"checks,omitempty"`
}

// Handler returns an http.Handler that runs every check and reports the
// aggregate result as JSON.
//
// Each check runs with its own context derived from the request's context
// and limited by timeout, so a client that disconnects cancels the checks and
// one slow dependency cannot exceed its budget. The response is 200 when all
// checks pass (or there are none) and 503 Service Unavailable otherwise.
func Handler(timeout time.Duration, checks map[string]Check) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: implement health check handler
		_ = w
		_ = r
		_ = timeout
		_ = checks
	})
}
