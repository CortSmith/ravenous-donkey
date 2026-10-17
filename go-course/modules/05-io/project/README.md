# Project: Log Analyzer

## Overview

Build a command-line tool that parses HTTP access logs in Apache Combined Log Format, aggregates statistics, and outputs a report in text or JSON format. The tool must stream logs without loading the entire file into memory.

## Specification

### Input Format

Logs are in Apache Combined Log Format:

```
10.0.0.2 - - [11/Oct/2026:13:00:00 +0000] "GET /api/items/7 HTTP/1.1" 200 1024 0.034
```

Fields (in order):
1. **IP address** (client IP, possibly IPv6)
2. **ident** (always `-` in modern logs)
3. **userid** (user ID if authenticated, else `-`)
4. **timestamp** in the format `[11/Oct/2026:13:00:00 +0000]`
5. **request line** in double quotes: `METHOD PATH HTTP/VERSION` (e.g., `"GET / HTTP/1.1"`)
6. **status code** (HTTP status, e.g., 200, 404, 500)
7. **bytes sent** (response body size, or `-` if none)
8. **latency** (response time in seconds, as a decimal)

Malformed lines should be skipped with a warning printed to stderr.

### Parsing

Implement a `func Parse(line string) (*LogEntry, error)` that returns:

```go
type LogEntry struct {
    IP       string        // client IP
    UserID   string        // authenticated user, or empty if "-"
    Timestamp time.Time    // parsed timestamp (use time.Parse with a custom layout)
    Method   string        // HTTP method
    Path     string        // request path (without query string)
    Status   int           // HTTP status code
    Bytes    int64         // response body size, 0 if "-"
    Latency  time.Duration // response time
}

ts, _ := time.Parse("02/Jan/2006:15:04:05 -0700", "11/Oct/2026:13:00:00 +0000")
e := LogEntry{IP: "10.0.0.2", Timestamp: ts, Method: "GET", Path: "/api/items/7", Status: 200, Bytes: 1024, Latency: 34 * time.Millisecond}
fmt.Printf("%+v\n", e)
```

The timestamp format is: `02/Jan/2006:15:04:05 -0700` (day/month/year:hour:min:sec timezone).

To extract the path from a request line like `"GET /api/items?page=2 HTTP/1.1"`, strip the query string. Malformed request lines should error.

### Aggregation

Implement an `Analyzer` type with methods that return statistics without holding all logs in memory:

```go
// Runs YOUR loganalyzer package. The API to implement:
//
//	func NewAnalyzer() *Analyzer                      // empty, ready to process logs
//	func (a *Analyzer) ProcessLine(line string) error // parse one line, update the statistics
//	func (a *Analyzer) StatusCounts() []StatusCount   // status code counts, by count (descending)
//	func (a *Analyzer) TopPaths(n int) []PathCount    // top n paths, most-requested first
//	func (a *Analyzer) Latency() LatencyStats         // p50, p95 and max latency
//
// Exact percentiles on a stream require storing every latency, which bends
// the "streaming" constraint; a production tool would estimate them with a
// sketch. For this project, store the latencies (8 bytes each) and compute
// exact percentiles.
import "gocourse/modules/05-io/project/loganalyzer"

var lines = []string{
	`10.0.0.2 - - [11/Oct/2026:13:00:00 +0000] "GET /api/items/7 HTTP/1.1" 200 1024 0.034`,
	`10.0.0.3 - ada [11/Oct/2026:13:00:01 +0000] "GET /api/items?page=2 HTTP/1.1" 200 512 0.120`,
	`10.0.0.2 - - [11/Oct/2026:13:00:02 +0000] "POST /api/items HTTP/1.1" 500 - 1.500`,
	`not a log line`,
}

func main() {
	a := loganalyzer.NewAnalyzer()
	for _, line := range lines {
		if err := a.ProcessLine(line); err != nil {
			fmt.Println("skipped:", err)
		}
	}
	fmt.Println(a.StatusCounts())
	fmt.Println(a.TopPaths(2))
	fmt.Printf("%+v\n", a.Latency())
}
```

Supporting types:

```go
type StatusCount struct {
    Status int
    Count  int
}

type PathCount struct {
    Path  string
    Count int
}

type LatencyStats struct {
    P50 time.Duration
    P95 time.Duration
    Max time.Duration
}

fmt.Println([]StatusCount{{200, 41}, {404, 3}}, []PathCount{{"/", 12}})
fmt.Printf("%+v\n", LatencyStats{P50: 30 * time.Millisecond, P95: 180 * time.Millisecond, Max: time.Second})
```

### Output Formats

#### Text Format

Plain text report, one metric per line or table:

```
Status Codes:
  200: 45
  404: 12
  500: 3

Top Paths:
  /api/items: 30
  /static/style.css: 15
  /: 10

Latency:
  p50: 45ms
  p95: 120ms
  max: 500ms
```

#### JSON Format

Structured JSON output:

```json
{
  "status_codes": [
    {"status": 200, "count": 45},
    {"status": 404, "count": 12},
    {"status": 500, "count": 3}
  ],
  "top_paths": [
    {"path": "/api/items", "count": 30},
    {"path": "/static/style.css", "count": 15},
    {"path": "/", "count": 10}
  ],
  "latency": {
    "p50_ms": 45,
    "p95_ms": 120,
    "max_ms": 500
  }
}
```

### CLI

Implement `cmd/loganalyzer/main.go` with the following interface:

```bash
loganalyzer [flags] [file]
```

Flags:
- `-format text|json` (default: `text`) — output format
- `-top N` (default: 10) — number of top paths to show

Arguments:
- `[file]` — log file to analyze; if omitted or "-", read from stdin

Usage examples:

```bash
# Analyze a file
loganalyzer -format json access.log

# Analyze stdin
loganalyzer < access.log
loganalyzer - < access.log

# Show top 5 paths
loganalyzer -top 5 access.log
```

The tool should:
1. Accept a filename or stdin.
2. Read and parse logs with `bufio.Scanner` or a similar streaming approach.
3. Skip malformed lines and print warnings to stderr.
4. Output the report to stdout.
5. Exit with code 0 on success, 1 on error (other than parsing errors in logs).

### Streaming Requirement

The tool must not load the entire log file into memory. This means:
- Do not use `io.ReadAll` or `os.ReadFile`.
- Process logs one line at a time with `bufio.Scanner`.
- Update aggregate statistics incrementally.
- For latencies, storing all values is acceptable (they're few), but do not store raw log entries.

### Testing

Write tests that:
1. Test `Parse` with valid and malformed lines.
2. Test `Analyzer` with a small set of log lines.
3. Test the `main` function by calling it with a temporary test file and checking exit codes and output.
4. Include a test that processes the provided testdata and verifies the output format.

Use the provided `testdata/access.log` for integration tests.

### Acceptance Criteria

✓ Parses at least 90% of real-world Apache Combined Log Format lines correctly
✓ Skips malformed lines with a warning, does not crash
✓ Streams logs without loading entire file into memory
✓ Outputs text and JSON formats correctly
✓ All tests pass: `go test ./...` (add `-race` to check for concurrency issues)
✓ Code is `gofmt`-clean and passes `go vet`
✓ CLI respects `file` and `-` arguments to read from stdin

### Stretch Goals

- Add a `-latency-percentiles` flag to report custom percentiles (e.g., `p99`)
- Filter logs by IP or user: `-ip 10.0.0.2` or `-user bob`
- Output detailed stats per status code (e.g., average latency for 200 vs 500)
- Add color output for text reports (for terminals)
