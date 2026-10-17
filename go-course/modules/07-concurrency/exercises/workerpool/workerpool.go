// Package workerpool runs a function over a slice with a fixed number of
// worker goroutines, keeping results in input order and stopping at the
// first error.
package workerpool

import (
	"context"
	"errors"
)

// ErrInvalidWorkers is returned by Map when workers < 1.
var ErrInvalidWorkers = errors.New("workerpool: workers must be at least 1")

// Map calls fn on every element of in using at most workers goroutines and
// returns the results in the same order as in: out[i] = fn(ctx, in[i]).
//
// If any call fails, Map cancels the context passed to the calls still
// running, starts no further calls, waits for every worker to return, and
// returns (nil, err) with the first error. If ctx is done before all calls
// finish, Map returns (nil, context.Cause(ctx)), which is ctx.Err() unless the
// caller canceled with a cause. Map never leaves goroutines running after it
// returns.
func Map[In, Out any](ctx context.Context, in []In, workers int, fn func(context.Context, In) (Out, error)) ([]Out, error) {
	// TODO: implement
	return nil, ErrInvalidWorkers
}
