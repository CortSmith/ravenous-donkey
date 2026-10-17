package pipeline

import (
	"context"
	"sync"
)

// batchSize is how many numbers travel together through the fast pipeline.
const batchSize = 1024

// SumSquaresBatched has the same contract as SumSquares but moves work in
// batches: the generator sends sub-slices of nums (no copying), each worker
// squares and sums a whole batch locally, and only one partial sum per
// worker crosses a channel at the end. Channel operations drop from ~3 per
// number to ~1 per batchSize numbers.
func SumSquaresBatched(ctx context.Context, nums []int, workers int) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	batches := make(chan []int)
	go func() {
		defer close(batches)
		for lo := 0; lo < len(nums); lo += batchSize {
			select {
			case batches <- nums[lo:min(lo+batchSize, len(nums))]:
			case <-ctx.Done():
				return
			}
		}
	}()

	workers = max(workers, 1)
	partials := make(chan int, workers) // buffered: workers never block on their final send
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sum := 0
			for b := range batches {
				if ctx.Err() != nil {
					return
				}
				for _, v := range b {
					sum += v * v
				}
			}
			partials <- sum
		}()
	}
	wg.Wait()
	close(partials)

	if err := ctx.Err(); err != nil {
		return 0, err
	}
	total := 0
	for p := range partials {
		total += p
	}
	return total, nil
}
