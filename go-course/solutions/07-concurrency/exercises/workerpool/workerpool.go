// Package workerpool runs a function over a slice with a fixed number of
// worker goroutines, keeping results in input order and stopping at the
// first error.
package workerpool

import (
	"context"
	"errors"
	"sync"
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
	if workers < 1 {
		return nil, ErrInvalidWorkers
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	out := make([]Out, len(in))
	jobs := make(chan int) // indices into in; unbuffered so no job is "claimed" early

	var wg sync.WaitGroup
	for range min(workers, len(in)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				v, err := fn(ctx, in[i])
				if err != nil {
					cancel(err) // only the first cause sticks; later calls are no-ops
					return
				}
				out[i] = v // each index is written by exactly one worker
			}
		}()
	}

	// The calling goroutine is the producer, so it owns (and closes) jobs.
feed:
	for i := range in {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
