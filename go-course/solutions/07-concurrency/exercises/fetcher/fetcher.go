// Package fetcher fetches many URLs concurrently while bounding how many
// fetches are in flight at once.
package fetcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// FetchFunc fetches one URL. Real code would wrap an *http.Client; tests
// pass fakes. Implementations should return promptly when ctx is done.
type FetchFunc func(ctx context.Context, url string) (string, error)

// Result is the outcome of fetching one URL. Exactly one of Body and Err is
// meaningful: Err is nil on success.
type Result struct {
	URL  string
	Body string
	Err  error
}

// FetchAll fetches every URL in urls with at most limit calls to fetch in
// flight at any moment (limit < 1 is treated as 1). It returns one Result per
// URL, in the same order as urls. A failed fetch does not stop the others.
//
// If ctx is canceled, FetchAll starts no new fetches; the Result for every
// URL that was never started has Err == ctx.Err(). FetchAll waits for all
// started fetches to return before it returns.
func FetchAll(ctx context.Context, urls []string, limit int, fetch FetchFunc) []Result {
	results := make([]Result, len(urls))
	sem := make(chan struct{}, max(limit, 1)) // a token per in-flight fetch

	var wg sync.WaitGroup
	for i, u := range urls {
		results[i].URL = u

		// Acquire in the parent: this bounds the number of goroutines, not
		// just the number of busy ones, and lets us stop launching early.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		// select picks randomly when both cases are ready, so re-check:
		// never start a fetch once ctx is done.
		if err := ctx.Err(); err != nil {
			for j := i; j < len(urls); j++ {
				results[j] = Result{URL: urls[j], Err: err}
			}
			break
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }() // release
			body, err := fetch(ctx, u)
			results[i].Body, results[i].Err = body, err // each goroutine owns one element
		}()
	}
	wg.Wait()
	return results
}

// Errors combines the failures in results into one error, each wrapped with
// its URL, using errors.Join. It returns nil if every fetch succeeded.
func Errors(results []Result) error {
	var errs []error
	for _, r := range results {
		if r.Err != nil {
			errs = append(errs, fmt.Errorf("fetch %s: %w", r.URL, r.Err))
		}
	}
	return errors.Join(errs...) // nil when errs is empty
}
