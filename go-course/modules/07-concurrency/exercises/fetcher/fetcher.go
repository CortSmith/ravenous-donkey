// Package fetcher fetches many URLs concurrently while bounding how many
// fetches are in flight at once.
package fetcher

import (
	"context"
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
	// TODO: implement
	return nil
}

// Errors combines the failures in results into one error, each wrapped with
// its URL, using errors.Join. It returns nil if every fetch succeeded.
func Errors(results []Result) error {
	// TODO: implement
	return nil
}
