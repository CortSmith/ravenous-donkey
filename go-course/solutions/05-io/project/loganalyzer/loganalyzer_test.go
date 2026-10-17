package loganalyzer

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    *LogEntry
		wantErr bool
	}{
		{
			name: "valid simple line",
			line: `10.0.0.2 - - [11/Oct/2026:13:00:00 +0000] "GET / HTTP/1.1" 200 1024 0.034`,
			want: &LogEntry{
				IP:        "10.0.0.2",
				UserID:    "",
				Timestamp: mustParse("11/Oct/2026:13:00:00 +0000"),
				Method:    "GET",
				Path:      "/",
				Status:    200,
				Bytes:     1024,
				Latency:   34 * time.Millisecond,
			},
		},
		{
			name: "with authenticated user",
			line: `10.0.0.3 - alice [11/Oct/2026:13:01:00 +0000] "POST /api/items HTTP/1.1" 201 5000 0.234`,
			want: &LogEntry{
				IP:        "10.0.0.3",
				UserID:    "alice",
				Timestamp: mustParse("11/Oct/2026:13:01:00 +0000"),
				Method:    "POST",
				Path:      "/api/items",
				Status:    201,
				Bytes:     5000,
				Latency:   234 * time.Millisecond,
			},
		},
		{
			name: "ipv6 address",
			line: `2001:db8::7 - - [11/Oct/2026:13:02:00 +0000] "GET /static/style.css HTTP/1.1" 200 9693 0.003`,
			want: &LogEntry{
				IP:        "2001:db8::7",
				UserID:    "",
				Timestamp: mustParse("11/Oct/2026:13:02:00 +0000"),
				Method:    "GET",
				Path:      "/static/style.css",
				Status:    200,
				Bytes:     9693,
				Latency:   3 * time.Millisecond,
			},
		},
		{
			name: "path with query string",
			line: `10.0.0.4 - - [11/Oct/2026:13:03:00 +0000] "GET /api/items?page=2 HTTP/1.1" 200 2048 0.100`,
			want: &LogEntry{
				IP:        "10.0.0.4",
				UserID:    "",
				Timestamp: mustParse("11/Oct/2026:13:03:00 +0000"),
				Method:    "GET",
				Path:      "/api/items",
				Status:    200,
				Bytes:     2048,
				Latency:   100 * time.Millisecond,
			},
		},
		{
			name: "bytes absent (dash)",
			line: `10.0.0.5 - - [11/Oct/2026:13:04:00 +0000] "DELETE /resource HTTP/1.1" 204 - 0.050`,
			want: &LogEntry{
				IP:        "10.0.0.5",
				UserID:    "",
				Timestamp: mustParse("11/Oct/2026:13:04:00 +0000"),
				Method:    "DELETE",
				Path:      "/resource",
				Status:    204,
				Bytes:     0,
				Latency:   50 * time.Millisecond,
			},
		},
		{
			name:    "missing request line",
			line:    `10.0.0.2 - - [11/Oct/2026:13:00:00 +0000] 200 1024 0.034`,
			wantErr: true,
		},
		{
			name:    "malformed timestamp",
			line:    `10.0.0.2 - - [not-a-timestamp] "GET / HTTP/1.1" 200 1024 0.034`,
			wantErr: true,
		},
		{
			name:    "non-numeric status",
			line:    `10.0.0.2 - - [11/Oct/2026:13:00:00 +0000] "GET / HTTP/1.1" bad 1024 0.034`,
			wantErr: true,
		},
		{
			name:    "non-numeric latency",
			line:    `10.0.0.2 - - [11/Oct/2026:13:00:00 +0000] "GET / HTTP/1.1" 200 1024 slow`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.line, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if got.IP != tt.want.IP || got.Method != tt.want.Method || got.Path != tt.want.Path ||
				got.Status != tt.want.Status || got.Bytes != tt.want.Bytes || got.Latency != tt.want.Latency {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.line, got, tt.want)
			}
		})
	}
}

func TestAnalyzer(t *testing.T) {
	a := NewAnalyzer()
	lines := []string{
		`10.0.0.2 - - [11/Oct/2026:13:00:00 +0000] "GET /api/items/7 HTTP/1.1" 200 1024 0.034`,
		`10.0.0.2 - - [11/Oct/2026:13:00:07 +0000] "GET /api/items?page=2 HTTP/1.1" 200 1009 1.045`,
		`10.0.0.4 - - [11/Oct/2026:13:00:14 +0000] "GET / HTTP/1.1" 200 498 0.124`,
		`10.0.0.3 - - [11/Oct/2026:13:00:21 +0000] "DELETE /api/items/42?page=2 HTTP/1.1" 204 - 1.779`,
		`10.0.0.5 - - [11/Oct/2026:13:01:28 +0000] "GET /index.html HTTP/1.1" 200 39675 0.213`,
		`10.0.0.2 - - [11/Oct/2026:13:01:35 +0000] "POST /api/items HTTP/1.1" 404 11692 1.764`,
		`10.0.0.2 - - [11/Oct/2026:13:05:20 +0000] "GET / HTTP/1.1" 200 27379 0.023`,
		`10.0.0.3 - - [11/Oct/2026:13:05:34 +0000] "DELETE /api/items/42 HTTP/1.1" 500 51662 0.057`,
	}

	for _, line := range lines {
		if err := a.ProcessLine(line); err != nil {
			t.Fatalf("ProcessLine(%q) error = %v", line, err)
		}
	}

	// Check status counts
	sc := a.StatusCounts()
	if len(sc) == 0 {
		t.Errorf("StatusCounts() = empty slice, want 3 statuses (200, 204, 404, 500)")
	}
	// StatusCounts should be sorted by count descending
	for i := 1; i < len(sc); i++ {
		if sc[i-1].Count < sc[i].Count {
			t.Errorf("StatusCounts not sorted by count descending: %v", sc)
			break
		}
	}

	// Check top paths
	tp := a.TopPaths(3)
	if len(tp) < 1 {
		t.Errorf("TopPaths(3) = empty slice")
	}
	for i := 1; i < len(tp); i++ {
		if tp[i-1].Count < tp[i].Count {
			t.Errorf("TopPaths not sorted by count descending: %v", tp)
			break
		}
	}
	if len(tp) > 3 {
		t.Errorf("TopPaths(3) returned %d paths, want at most 3", len(tp))
	}

	// Check latency stats
	lat := a.Latency()
	if lat.P50 == 0 || lat.P95 == 0 || lat.Max == 0 {
		t.Errorf("Latency() = {P50: %v, P95: %v, Max: %v}, all should be non-zero", lat.P50, lat.P95, lat.Max)
	}
	if lat.P50 > lat.P95 || lat.P95 > lat.Max {
		t.Errorf("Latency() percentiles out of order: P50=%v, P95=%v, Max=%v", lat.P50, lat.P95, lat.Max)
	}
}

func TestTopPathsLimit(t *testing.T) {
	a := NewAnalyzer()
	paths := []string{"/a", "/b", "/c", "/d", "/e"}
	for _, p := range paths {
		line := fmt.Sprintf(`10.0.0.1 - - [11/Oct/2026:13:00:00 +0000] "GET %s HTTP/1.1" 200 100 0.001`, p)
		a.ProcessLine(line)
	}

	tests := []struct {
		n    int
		want int
	}{
		{0, 0},
		{1, 1},
		{3, 3},
		{10, 5}, // more than available
		{-1, 0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("n=%d", tt.n), func(t *testing.T) {
			got := a.TopPaths(tt.n)
			if len(got) != tt.want {
				t.Errorf("TopPaths(%d) returned %d paths, want %d", tt.n, len(got), tt.want)
			}
		})
	}
}

func TestMalformedLines(t *testing.T) {
	a := NewAnalyzer()
	lines := []string{
		"this is not a log line",
		`10.0.0.1 - - [11/Oct/2026:13:00:00 +0000] "GET / HTTP/1.1" 200 100 0.001`,
		"",
		"incomplete line",
	}

	errCount := 0
	for _, line := range lines {
		err := a.ProcessLine(line)
		if err != nil {
			errCount++
		}
	}

	if errCount == 0 {
		t.Errorf("ProcessLine should have errors for malformed lines, got 0 errors")
	}
}

func TestEmptyAnalyzer(t *testing.T) {
	a := NewAnalyzer()

	sc := a.StatusCounts()
	if len(sc) != 0 {
		t.Errorf("StatusCounts() on empty analyzer = %v, want empty", sc)
	}

	tp := a.TopPaths(10)
	if len(tp) != 0 {
		t.Errorf("TopPaths(10) on empty analyzer = %v, want empty", tp)
	}

	lat := a.Latency()
	if lat.P50 != 0 || lat.P95 != 0 || lat.Max != 0 {
		t.Errorf("Latency() on empty analyzer = %v, want all zeros", lat)
	}
}

func TestIntegrationWithTestdata(t *testing.T) {
	// Load testdata/access.log (if it exists)
	dataByte, err := os.ReadFile("testdata/access.log")
	if err != nil {
		t.Skipf("testdata/access.log not found or unreadable: %v", err)
	}

	a := NewAnalyzer()
	for _, line := range strings.Split(string(dataByte), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Ignore errors from malformed lines (test data includes one)
		_ = a.ProcessLine(line)
	}

	// Just check that we got some results
	sc := a.StatusCounts()
	if len(sc) == 0 {
		t.Error("StatusCounts() after processing testdata returned empty")
	}

	tp := a.TopPaths(10)
	if len(tp) == 0 {
		t.Error("TopPaths(10) after processing testdata returned empty")
	}

	lat := a.Latency()
	if lat.Max == 0 {
		t.Error("Latency() after processing testdata returned all zeros")
	}
}

// mustParse parses a timestamp in Apache log format, panicking on error.
func mustParse(ts string) time.Time {
	t, err := time.Parse("02/Jan/2006:15:04:05 -0700", ts)
	if err != nil {
		panic(err)
	}
	return t
}
