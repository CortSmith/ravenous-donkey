package workerpool

import (
	"context"
	"sync"
	"sync/atomic"
)

// MapFast has the same contract as Map but hands out work differently:
// instead of a producer sending each index over an unbuffered channel (two
// goroutine handoffs per job), every worker claims the next index with one
// atomic add. There is no producer goroutine and no channel, which matters
// when fn is cheap and the per-job coordination cost dominates.
func MapFast[In, Out any](ctx context.Context, in []In, workers int, fn func(context.Context, In) (Out, error)) ([]Out, error) {
	if workers < 1 {
		return nil, ErrInvalidWorkers
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	out := make([]Out, len(in))
	var next atomic.Int64
	work := func() {
		for ctx.Err() == nil {
			i := int(next.Add(1) - 1)
			if i >= len(in) {
				return
			}
			v, err := fn(ctx, in[i])
			if err != nil {
				cancel(err)
				return
			}
			out[i] = v
		}
	}

	n := min(workers, len(in))
	var wg sync.WaitGroup
	for range n - 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			work()
		}()
	}
	if n > 0 {
		work() // the calling goroutine is one of the workers
	}
	wg.Wait()

	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
