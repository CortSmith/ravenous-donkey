package logstats

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"strings"
	"testing"
	"testing/iotest"
)

func TestSummarizeExample(t *testing.T) {
	in := `2026-10-11T12:00:00Z GET /api/users 200 12 5120
2026-10-11T12:00:01Z POST /api/users 201 40 128

2026-10-11T12:00:02Z GET /api/users 200 30 4096
2026-10-11T12:00:03Z GET /health 204 1 0
this line is garbage
2026-10-11T12:00:04Z GET /missing 404 3 9
`
	want := Summary{
		Lines:     6,
		Malformed: 1,
		Methods:   map[string]int{"GET": 4, "POST": 1},
		Status:    map[int]int{200: 2, 201: 1, 204: 1, 404: 1},
		Paths: map[string]PathStats{
			"/api/users": {Count: 3, TotalMillis: 82, MaxMillis: 40, Bytes: 9344},
			"/health":    {Count: 1, TotalMillis: 1, MaxMillis: 1, Bytes: 0},
			"/missing":   {Count: 1, TotalMillis: 3, MaxMillis: 3, Bytes: 9},
		},
	}
	got, err := Summarize(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}
	checkSummary(t, got, want)
}

func TestSummarizeEmpty(t *testing.T) {
	for _, in := range []string{"", "\n", "\n\n  \n\t\n", "\r\n \r\n"} {
		got, err := Summarize(strings.NewReader(in))
		if err != nil {
			t.Fatalf("Summarize(%q) returned error: %v", in, err)
		}
		if got.Methods == nil || got.Status == nil || got.Paths == nil {
			t.Errorf("Summarize(%q) returned nil maps: %+v; maps must never be nil", in, got)
		}
		checkSummary(t, got, Summary{Methods: map[string]int{}, Status: map[int]int{}, Paths: map[string]PathStats{}})
	}
}

// TestSummarizeWellFormed lists edge cases that must be accepted.
func TestSummarizeWellFormed(t *testing.T) {
	tests := []struct {
		name, line   string
		method, path string
		status       int
		dur, size    int64
	}{
		{"plain", "ts GET /a 200 1 2", "GET", "/a", 200, 1, 2},
		{"root path", "ts GET / 200 1 2", "GET", "/", 200, 1, 2},
		{"tabs", "ts\tPUT\t/a\t201\t5\t6", "PUT", "/a", 201, 5, 6},
		{"many separators", "ts   DELETE \t /x/y  204   0    0", "DELETE", "/x/y", 204, 0, 0},
		{"surrounding space", "  \t ts GET /a 200 1 2 \t ", "GET", "/a", 200, 1, 2},
		{"crlf", "ts GET /a 200 1 2\r", "GET", "/a", 200, 1, 2},
		{"status 100", "ts GET /a 100 1 2", "GET", "/a", 100, 1, 2},
		{"status 599", "ts GET /a 599 1 2", "GET", "/a", 599, 1, 2},
		{"leading zeros in numbers", "ts GET /a 200 007 0010", "GET", "/a", 200, 7, 10},
		{"max int64", "ts GET /a 200 9223372036854775807 9223372036854775807", "GET", "/a", 200, math.MaxInt64, math.MaxInt64},
		{"long method", "ts PROPFIND /dav 207 3 4", "PROPFIND", "/dav", 207, 3, 4},
		{"single letter method", "ts X /a 200 1 2", "X", "/a", 200, 1, 2},
		{"query in path", "ts GET /search?q=go&x=1 200 1 2", "GET", "/search?q=go&x=1", 200, 1, 2},
		{"unicode path", "ts GET /café 200 1 2", "GET", "/café", 200, 1, 2},
		{"non-ascii space is not a separator", "ts GET /a\u00a0b 200 1 2", "GET", "/a\u00a0b", 200, 1, 2},
		{"weird timestamp", "whatever GET /a 200 1 2", "GET", "/a", 200, 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Summarize(strings.NewReader(tt.line + "\n"))
			if err != nil {
				t.Fatalf("Summarize(%q) returned error: %v", tt.line, err)
			}
			want := Summary{
				Lines:   1,
				Methods: map[string]int{tt.method: 1},
				Status:  map[int]int{tt.status: 1},
				Paths:   map[string]PathStats{tt.path: {Count: 1, TotalMillis: tt.dur, MaxMillis: tt.dur, Bytes: tt.size}},
			}
			checkSummary(t, got, want)
		})
	}
}

// TestSummarizeMalformed lists lines that must be counted as Malformed.
func TestSummarizeMalformed(t *testing.T) {
	tests := []struct{ name, line string }{
		{"too few fields", "ts GET /a 200 1"},
		{"too many fields", "ts GET /a 200 1 2 3"},
		{"one field", "garbage"},
		{"lower-case method", "ts get /a 200 1 2"},
		{"mixed-case method", "ts Get /a 200 1 2"},
		{"digit in method", "ts GET2 /a 200 1 2"},
		{"path without slash", "ts GET a 200 1 2"},
		{"status too small", "ts GET /a 099 1 2"},
		{"status too large", "ts GET /a 600 1 2"},
		{"status two digits", "ts GET /a 20 1 2"},
		{"status four digits", "ts GET /a 2000 1 2"},
		{"status not a number", "ts GET /a OK 1 2"},
		{"status with sign", "ts GET /a +200 1 2"},
		{"negative duration", "ts GET /a 200 -1 2"},
		{"plus duration", "ts GET /a 200 +1 2"},
		{"duration not a number", "ts GET /a 200 1ms 2"},
		{"bytes not a number", "ts GET /a 200 1 2kb"},
		{"duration overflow", "ts GET /a 200 9223372036854775808 2"},
		{"bytes overflow", "ts GET /a 200 1 99999999999999999999"},
		{"comma separated", "ts,GET,/a,200,1,2"},
		{"non-ascii space glues method to path", "ts GET\u00a0/a 200 1 2 3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Summarize(strings.NewReader(tt.line + "\n"))
			if err != nil {
				t.Fatalf("Summarize(%q) returned error: %v", tt.line, err)
			}
			want := Summary{Lines: 1, Malformed: 1, Methods: map[string]int{}, Status: map[int]int{}, Paths: map[string]PathStats{}}
			checkSummary(t, got, want)
		})
	}
}

func TestSummarizeNoTrailingNewline(t *testing.T) {
	got, err := Summarize(strings.NewReader("ts GET /a 200 1 2\nts GET /a 200 3 4"))
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}
	if got.Lines != 2 || got.Paths["/a"].Count != 2 {
		t.Errorf("Summarize without final newline: Lines=%d, /a count=%d; want 2, 2", got.Lines, got.Paths["/a"].Count)
	}
}

func TestSummarizeGenerated(t *testing.T) {
	data, want := generate(20000)
	got, err := Summarize(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}
	checkSummary(t, got, want)
}

// TestSummarizeSmallReads feeds the input one byte at a time, which catches
// implementations that assume a Read returns whole lines.
func TestSummarizeSmallReads(t *testing.T) {
	data, want := generate(500)
	got, err := Summarize(iotest.OneByteReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}
	checkSummary(t, got, want)
}

func TestSummarizeReadError(t *testing.T) {
	errBoom := errors.New("boom")
	r := io.MultiReader(strings.NewReader("ts GET /a 200 1 2\n"), iotest.ErrReader(errBoom))
	got, err := Summarize(r)
	if !errors.Is(err, errBoom) {
		t.Fatalf("Summarize error = %v, want it to wrap %v", err, errBoom)
	}
	if got.Lines != 1 || got.Paths["/a"].Count != 1 {
		t.Errorf("Summarize on read error returned %+v, want the line read before the error to be counted", got)
	}
}

func TestSummarizeLineTooLong(t *testing.T) {
	long := "ts GET /" + strings.Repeat("a", 100_000) + " 200 1 2\n"
	if _, err := Summarize(strings.NewReader(long)); err == nil {
		t.Errorf("Summarize(line of %d bytes) returned nil error, want an error", len(long))
	}
}

// TestSummarizeAllocs is the performance gate. With 8 distinct paths, 4
// methods and 6 status codes, the work per line can be done without
// allocating at all; only building the result maps and remembering each
// distinct key needs memory.
func TestSummarizeAllocs(t *testing.T) {
	data, want := generate(2000)
	rd := bytes.NewReader(data)
	allocs := testing.AllocsPerRun(20, func() {
		rd.Reset(data)
		got, err := Summarize(rd)
		if err != nil || got.Lines != want.Lines {
			t.Fatalf("Summarize = %d lines, %v; want %d lines, nil", got.Lines, err, want.Lines)
		}
	})
	const maxAllocs = 60
	if allocs > maxAllocs {
		t.Errorf("Summarize(2000 lines) allocates %.0f times per call, want <= %d.\n"+
			"Profile it: go test -run='^$' -bench=Summarize -memprofile=mem.out ./modules/09-performance/project/\n"+
			"            go tool pprof -sample_index=alloc_objects -top mem.out", allocs, maxAllocs)
	}
}

// TestSummarizeAllocsDoNotScale checks that allocations depend on the number
// of distinct keys, not on the number of lines: 4x the lines (same keys) must
// not mean more allocations.
func TestSummarizeAllocsDoNotScale(t *testing.T) {
	measure := func(lines int) float64 {
		data, _ := generate(lines)
		rd := bytes.NewReader(data)
		return testing.AllocsPerRun(10, func() {
			rd.Reset(data)
			if _, err := Summarize(rd); err != nil {
				t.Fatalf("Summarize returned error: %v", err)
			}
		})
	}
	small, large := measure(1000), measure(4000)
	if large > small+2 {
		t.Errorf("Summarize allocates %.0f times for 1000 lines but %.0f for 4000 lines; "+
			"allocations should not grow with the number of lines (look for per-line allocations, "+
			"including on malformed lines)", small, large)
	}
}

func BenchmarkSummarize(b *testing.B) {
	data, _ := generate(10000)
	rd := bytes.NewReader(data)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		rd.Reset(data)
		if _, err := Summarize(rd); err != nil {
			b.Fatal(err)
		}
	}
}

// checkSummary compares two Summaries and reports differences readably.
func checkSummary(t *testing.T, got, want Summary) {
	t.Helper()
	if got.Lines != want.Lines || got.Malformed != want.Malformed {
		t.Errorf("Lines, Malformed = %d, %d; want %d, %d", got.Lines, got.Malformed, want.Lines, want.Malformed)
	}
	if !maps.Equal(got.Methods, want.Methods) {
		t.Errorf("Methods = %v, want %v", got.Methods, want.Methods)
	}
	if !maps.Equal(got.Status, want.Status) {
		t.Errorf("Status = %v, want %v", got.Status, want.Status)
	}
	if !maps.Equal(got.Paths, want.Paths) {
		t.Errorf("Paths differ:\n got %+v\nwant %+v", got.Paths, want.Paths)
	}
}

var (
	genMethods = []string{"GET", "GET", "GET", "POST", "PUT", "DELETE"}
	genPaths   = []string{"/", "/api/users", "/api/users/42", "/api/orders", "/static/app.js", "/login", "/health", "/api/search?q=go"}
	genStatus  = []int{200, 200, 200, 201, 204, 301, 404, 500}
	genBad     = []string{
		"GET /api/users 200 12 5120",           // missing timestamp
		"ts GET /api/users 200 12",             // missing bytes
		"ts get /api/users 200 12 5120",        // lower-case method
		"ts GET api/users 200 12 5120",         // no leading slash
		"ts GET /api/users 700 12 5120",        // bad status
		"ts GET /api/users 200 -12 5120",       // negative duration
		"ts GET /api/users 200 12 lots",        // bad bytes
		"ts GET /api/users 200 12 5120 extra",  // too many fields
		"panic: runtime error: index out of r", // junk
	}
)

// generate returns a deterministic log of n lines (plus some blank lines) and
// the Summary that Summarize must produce for it. Every method, path and
// status appears within the first few hundred lines, so any prefix longer
// than that has the same set of distinct keys.
func generate(n int) ([]byte, Summary) {
	want := Summary{Methods: map[string]int{}, Status: map[int]int{}, Paths: map[string]PathStats{}}
	var buf bytes.Buffer
	x := uint32(2463534242)
	next := func() uint32 { // xorshift32: fast, deterministic, good enough
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		return x
	}
	for i := range n {
		if i%50 == 49 {
			buf.WriteString("   \n") // blank line: not counted
		}
		want.Lines++
		if i%37 == 36 {
			buf.WriteString(genBad[(i/37)%len(genBad)])
			buf.WriteByte('\n')
			want.Malformed++
			continue
		}
		method := genMethods[i%len(genMethods)]
		path := genPaths[(i/len(genMethods))%len(genPaths)]
		status := genStatus[i%len(genStatus)]
		dur := int64(next() % 2000)
		size := int64(next() % 100_000)
		fmt.Fprintf(&buf, "2026-10-11T12:%02d:%02dZ %s %s %d %d %d\n", (i/60)%60, i%60, method, path, status, dur, size)
		want.Methods[method]++
		want.Status[status]++
		ps := want.Paths[path]
		ps.Count++
		ps.TotalMillis += dur
		ps.MaxMillis = max(ps.MaxMillis, dur)
		ps.Bytes += size
		want.Paths[path] = ps
	}
	return buf.Bytes(), want
}
