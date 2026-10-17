// Package pipeline builds a cancellable generator -> square -> merge -> sum
// pipeline out of goroutines and channels.
//
// Every stage follows the same rules (https://go.dev/blog/pipelines):
//   - the stage owns its output channel and closes it when it returns;
//   - every send and every receive also selects on ctx.Done(), so a stage
//     can always exit when the context is canceled, even if nobody is reading
//     its output or nobody is writing its input.
package pipeline

import (
	"context"
	"sync"
)

// Generate returns a channel that yields nums in order and is then closed.
// If ctx is canceled first, the channel is closed early.
func Generate(ctx context.Context, nums ...int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for _, n := range nums {
			select {
			case out <- n:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// Square returns a channel that yields v*v for each v received from in, in
// the same order. The returned channel is closed when in is closed or when
// ctx is canceled, whichever happens first.
func Square(ctx context.Context, in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for {
			var v int
			select {
			case x, ok := <-in:
				if !ok {
					return
				}
				v = x
			case <-ctx.Done():
				return
			}
			select {
			case out <- v * v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// Merge fans in: it returns a channel that yields every value received from
// each of cs (in no particular order) and is closed once all of cs are
// closed, or once ctx is canceled. Merge with no channels returns a closed
// channel.
func Merge(ctx context.Context, cs ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup
	for _, c := range cs {
		wg.Add(1)
		go func() { // c is per-iteration since Go 1.22; no need to pass it in
			defer wg.Done()
			for {
				select {
				case v, ok := <-c:
					if !ok {
						return
					}
					select {
					case out <- v:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	// Several goroutines send on out, so none of them may close it. A
	// separate goroutine closes it once they have all finished.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// SumSquares returns the sum of the squares of nums. It runs one Generate
// stage, fans out to workers Square stages that all read from it, merges
// their outputs, and sums the merged values. workers < 1 is treated as 1.
//
// If ctx is canceled before the sum is complete, SumSquares returns
// (0, ctx.Err()) and all of its goroutines exit.
func SumSquares(ctx context.Context, nums []int, workers int) (int, error) {
	// A private child context lets us shut the whole pipeline down on any
	// return path, even though we normally drain it to completion.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	src := Generate(ctx, nums...)
	squares := make([]<-chan int, max(workers, 1))
	for i := range squares {
		squares[i] = Square(ctx, src)
	}

	total := 0
	for v := range Merge(ctx, squares...) {
		total += v
	}
	// The merged channel closes both on success and on cancellation, so we
	// must check which one happened.
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return total, nil
}
