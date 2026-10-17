// Package loganalyzer parses and aggregates HTTP access logs.
package loganalyzer

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
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
// The format is: IP IDENT USERID [TIMESTAMP] "METHOD PATH HTTP/VERSION" STATUS BYTES LATENCY
func Parse(line string) (*LogEntry, error) {
	// Find timestamp brackets
	bracketStart := strings.IndexByte(line, '[')
	if bracketStart == -1 {
		return nil, errors.New("parse: missing timestamp bracket")
	}
	bracketEnd := strings.IndexByte(line[bracketStart:], ']')
	if bracketEnd == -1 {
		return nil, errors.New("parse: missing closing timestamp bracket")
	}
	bracketEnd += bracketStart

	// Find quoted request line
	quoteStart := strings.IndexByte(line[bracketEnd:], '"')
	if quoteStart == -1 {
		return nil, errors.New("parse: missing request line quote")
	}
	quoteStart += bracketEnd
	quoteEnd := strings.IndexByte(line[quoteStart+1:], '"')
	if quoteEnd == -1 {
		return nil, errors.New("parse: missing closing request line quote")
	}
	quoteEnd += quoteStart + 1

	// Parse IP and user info
	beforeTS := strings.Fields(line[:bracketStart])
	if len(beforeTS) < 3 {
		return nil, errors.New("parse: not enough fields before timestamp")
	}

	ip := beforeTS[0]
	userid := beforeTS[2]
	if userid == "-" {
		userid = ""
	}

	// Parse timestamp
	tsStr := line[bracketStart+1 : bracketEnd]
	ts, err := time.Parse("02/Jan/2006:15:04:05 -0700", tsStr)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp %q: %w", tsStr, err)
	}

	// Parse request line
	reqStr := line[quoteStart+1 : quoteEnd]
	reqParts := strings.Fields(reqStr)
	if len(reqParts) != 3 {
		return nil, fmt.Errorf("parse: malformed request line %q", reqStr)
	}

	method := reqParts[0]
	pathWithQuery := reqParts[1]
	path, _, _ := strings.Cut(pathWithQuery, "?")

	// Parse status, bytes, latency
	afterReq := strings.Fields(line[quoteEnd+1:])
	if len(afterReq) < 3 {
		return nil, errors.New("parse: not enough fields after request")
	}

	status, err := strconv.Atoi(afterReq[0])
	if err != nil {
		return nil, fmt.Errorf("parse status %q: %w", afterReq[0], err)
	}

	var bytes int64
	if afterReq[1] != "-" {
		b, err := strconv.ParseInt(afterReq[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse bytes %q: %w", afterReq[1], err)
		}
		bytes = b
	}

	latencyFloat, err := strconv.ParseFloat(afterReq[2], 64)
	if err != nil {
		return nil, fmt.Errorf("parse latency %q: %w", afterReq[2], err)
	}
	latency := time.Duration(latencyFloat * float64(time.Second))

	return &LogEntry{
		IP:        ip,
		UserID:    userid,
		Timestamp: ts,
		Method:    method,
		Path:      path,
		Status:    status,
		Bytes:     bytes,
		Latency:   latency,
	}, nil
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
type Analyzer struct {
	statusCounts map[int]int
	pathCounts   map[string]int
	latencies    []time.Duration
}

// NewAnalyzer returns an empty Analyzer ready to process logs.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		statusCounts: make(map[int]int),
		pathCounts:   make(map[string]int),
		latencies:    []time.Duration{},
	}
}

// ProcessLine reads one log line and updates internal statistics.
func (a *Analyzer) ProcessLine(line string) error {
	entry, err := Parse(line)
	if err != nil {
		return err
	}
	a.statusCounts[entry.Status]++
	a.pathCounts[entry.Path]++
	a.latencies = append(a.latencies, entry.Latency)
	return nil
}

// StatusCounts returns a slice of status codes and their counts,
// sorted by count in descending order.
func (a *Analyzer) StatusCounts() []StatusCount {
	var counts []StatusCount
	for status, count := range a.statusCounts {
		counts = append(counts, StatusCount{Status: status, Count: count})
	}
	slices.SortFunc(counts, func(x, y StatusCount) int {
		return y.Count - x.Count
	})
	return counts
}

// TopPaths returns the top n paths by request count, most-requested first.
func (a *Analyzer) TopPaths(n int) []PathCount {
	if n <= 0 {
		return []PathCount{}
	}
	var paths []PathCount
	for path, count := range a.pathCounts {
		paths = append(paths, PathCount{Path: path, Count: count})
	}
	slices.SortFunc(paths, func(x, y PathCount) int {
		return y.Count - x.Count
	})
	if n < len(paths) {
		paths = paths[:n]
	}
	return paths
}

// Latency returns p50, p95, and max latencies.
func (a *Analyzer) Latency() LatencyStats {
	if len(a.latencies) == 0 {
		return LatencyStats{}
	}
	slices.Sort(a.latencies)
	n := len(a.latencies)
	return LatencyStats{
		P50: a.latencies[(n-1)*50/100],
		P95: a.latencies[(n-1)*95/100],
		Max: a.latencies[n-1],
	}
}

// FilterByIP returns a filtered view of paths for a specific IP. For use in stretch goals.
func (a *Analyzer) FilterByIP(ip string) *Analyzer {
	// This would require storing entries by IP; not implemented in basic version.
	_ = ip
	return a
}

// DefaultPort extracts the port from a "host:port" string, returning the port
// or an error if the format is invalid.
func DefaultPort(listen string) (int, error) {
	_, portStr, err := net.SplitHostPort(listen)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, err
	}
	return port, nil
}
