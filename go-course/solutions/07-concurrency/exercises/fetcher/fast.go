package fetcher

import (
	"context"
	"sync"
	"sync/atomic"
)

// FetchAllPool has the same contract as FetchAll but starts exactly
// min(limit, len(urls)) long-lived workers that claim URLs with an atomic
// counter, instead of one short-lived goroutine plus two semaphore channel
// operations per URL. For large URL lists this means fewer goroutine
// creations and no semaphore traffic. For real network fetches the
// difference is noise; the network dominates.
func FetchAllPool(ctx context.Context, urls []string, limit int, fetch FetchFunc) []Result {
	results := make([]Result, len(urls))
	var next atomic.Int64
	var wg sync.WaitGroup
	for range min(max(limit, 1), len(urls)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1) - 1)
				if i >= len(urls) {
					return
				}
				r := &results[i]
				r.URL = urls[i]
				if err := ctx.Err(); err != nil {
					r.Err = err
					continue // keep claiming so every remaining Result is filled in
				}
				r.Body, r.Err = fetch(ctx, urls[i])
			}
		}()
	}
	wg.Wait()
	return results
}
