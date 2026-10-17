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
)

// Generate returns a channel that yields nums in order and is then closed.
// If ctx is canceled first, the channel is closed early.
func Generate(ctx context.Context, nums ...int) <-chan int {
	// TODO: implement
	return nil
}

// Square returns a channel that yields v*v for each v received from in, in
// the same order. The returned channel is closed when in is closed or when
// ctx is canceled, whichever happens first.
func Square(ctx context.Context, in <-chan int) <-chan int {
	// TODO: implement
	return nil
}

// Merge fans in: it returns a channel that yields every value received from
// each of cs (in no particular order) and is closed once all of cs are
// closed, or once ctx is canceled. Merge with no channels returns a closed
// channel.
func Merge(ctx context.Context, cs ...<-chan int) <-chan int {
	// TODO: implement
	return nil
}

// SumSquares returns the sum of the squares of nums. It runs one Generate
// stage, fans out to workers Square stages that all read from it, merges
// their outputs, and sums the merged values. workers < 1 is treated as 1.
//
// If ctx is canceled before the sum is complete, SumSquares returns
// (0, ctx.Err()) and all of its goroutines exit.
func SumSquares(ctx context.Context, nums []int, workers int) (int, error) {
	// TODO: implement
	return 0, context.Canceled
}
