// Package health implements a JSON health-check endpoint whose individual
// checks are bounded by a per-check deadline derived from the request context.
package health

import (
	"context"
	"encoding/json"
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
		resp := Response{Status: "ok", Checks: make(map[string]string, len(checks))}
		for name, check := range checks {
			if err := runCheck(r.Context(), timeout, check); err != nil {
				resp.Status = "fail"
				resp.Checks[name] = err.Error()
				continue
			}
			resp.Checks[name] = "ok"
		}

		status := http.StatusOK
		if resp.Status != "ok" {
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		// Headers are already sent; an encoding error here can only mean the
		// client went away, so there is nothing useful left to do with it.
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// runCheck runs one check under its own deadline. Keeping it in a separate
// function lets defer cancel() release the timer as soon as the check returns
// instead of piling up defers until the handler ends.
func runCheck(parent context.Context, timeout time.Duration, check Check) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return check(ctx)
}
