// Package loganalyzer parses and aggregates HTTP access logs.
package loganalyzer

import (
	"errors"
	"time"
)

// LogEntry represents one line from an HTTP access log.
type LogEntry struct {
	IP        string        // client IP
	UserID    string        // authenticated user, or empty if "-"
	Timestamp time.Time     // parsed timestamp
	Method    string        // HTTP method (GET, POST, etc.)
	Path      string        // request path (without query string)
	Status    int           // HTTP status code
	Bytes     int64         // response body size, 0 if "-"
	Latency   time.Duration // response time
}

// Parse decodes one line of Apache Combined Log Format.
//
// The format is:
//
//	IP IDENT USERID [TIMESTAMP] "METHOD PATH HTTP/VERSION" STATUS BYTES LATENCY
//
// TODO: implement. Hints:
//   - Use strings.FieldsFunc or manual parsing to split by spaces, but be careful
//     with quoted fields like the request line.
//   - The timestamp format is: 02/Jan/2006:15:04:05 -0700
//   - To extract the path from "GET /api/items?page=2 HTTP/1.1", parse the request
//     line and strip the query string using strings.IndexByte.
//   - Return specific errors for different parsing failures, wrapped with context.
func Parse(line string) (*LogEntry, error) {
	return nil, errors.New("not implemented")
}

// StatusCount represents a status code and its count.
type StatusCount struct {
	Status int
	Count  int
}

// PathCount represents a path and its request count.
type PathCount struct {
	Path  string
	Count int
}

// LatencyStats reports latency percentiles.
type LatencyStats struct {
	P50 time.Duration
	P95 time.Duration
	Max time.Duration
}

// Analyzer aggregates log statistics.
//
// TODO: define unexported fields to track:
// - count of each status code
// - count of each path
// - all latencies (needed to compute percentiles)
type Analyzer struct {
	// TODO: add fields
}

// NewAnalyzer returns an empty Analyzer ready to process logs.
//
// TODO: implement. Initialize all fields.
func NewAnalyzer() *Analyzer {
	return nil
}

// ProcessLine reads one log line and updates internal statistics.
//
// TODO: implement. Call Parse to decode the line, then update the statistics.
// Return the error from Parse if parsing fails.
func (a *Analyzer) ProcessLine(line string) error {
	return errors.New("not implemented")
}

// StatusCounts returns a slice of status codes and their counts,
// sorted by count in descending order.
//
// TODO: implement. Return a []StatusCount sorted by Count (highest first).
// Use slices.SortFunc if needed.
func (a *Analyzer) StatusCounts() []StatusCount {
	return nil
}

// TopPaths returns the top n paths by request count, most-requested first.
//
// TODO: implement. Return a []PathCount with the n most-frequent paths,
// sorted by Count (highest first). If there are fewer than n unique paths,
// return all of them. Return an empty slice if n <= 0.
func (a *Analyzer) TopPaths(n int) []PathCount {
	return nil
}

// Latency returns p50, p95, and max latencies.
//
// TODO: implement. Sort the latencies and compute percentiles.
// - p50 is the median (50th percentile)
// - p95 is the 95th percentile
// - max is the maximum value
//
// Hint: For a sorted slice of length n, the p50 value is at index (n-1)*50/100,
// and p95 is at index (n-1)*95/100. Use integer division and be careful with
// empty slices.
func (a *Analyzer) Latency() LatencyStats {
	return LatencyStats{}
}
