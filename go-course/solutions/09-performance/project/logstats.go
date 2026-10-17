// Package logstats summarizes HTTP access logs.
//
// Each log line has six fields separated by one or more spaces or tabs:
//
//	<timestamp> <method> <path> <status> <duration_ms> <bytes>
//	2026-10-11T12:00:00Z GET /api/users 200 12 5120
//
// The timestamp is treated as an opaque token (it is not validated).
package logstats

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
)

// PathStats aggregates the requests for one path.
type PathStats struct {
	Count       int   // number of well-formed requests for the path
	TotalMillis int64 // sum of durations
	MaxMillis   int64 // largest single duration
	Bytes       int64 // sum of response sizes
}

// Summary is the result of Summarize.
type Summary struct {
	Lines     int                  // non-blank lines seen (well-formed + malformed)
	Malformed int                  // lines that failed validation
	Methods   map[string]int       // method -> count, well-formed lines only
	Status    map[int]int          // status code -> count, well-formed lines only
	Paths     map[string]PathStats // path -> stats, well-formed lines only
}

// entry is one parsed log line. method and path alias the scanner's buffer
// and are only valid until the next call to Scan.
type entry struct {
	method   []byte
	path     []byte
	status   int
	duration int64
	bytes    int64
}

// Summarize reads an access log from r and aggregates it.
//
// Leading and trailing spaces, tabs and carriage returns are ignored, and
// lines that are then empty are skipped and not counted. Only ' ' and '\t'
// separate fields (other whitespace is part of a field). A non-blank line
// is well-formed when it has exactly 6 fields, the method is
// one or more ASCII upper-case letters, the path starts with '/', the status
// is three digits in 100-599, and duration and bytes are unsigned decimal
// integers that fit in an int64. Any other non-blank line counts as
// Malformed and is otherwise ignored.
//
// The maps in the returned Summary are never nil. Summarize returns an error
// only if reading fails (including a line longer than bufio.MaxScanTokenSize);
// the Summary of the lines read so far is returned with it.
//
// Per line, Summarize does not allocate: it works on sc.Bytes() (a view into
// the scanner's buffer), splits and parses fields by hand, and looks up
// map entries with m[string(b)], which the compiler performs without
// converting b. Memory is only allocated for each distinct method and path
// (the first time it is seen) and for the result maps.
func Summarize(r io.Reader) (Summary, error) {
	var (
		methods = make(map[string]*int)
		paths   = make(map[string]*PathStats)
		status  [600]int // indexed by status code; validation guarantees 100-599
		lines   int
		bad     int
	)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := bytes.Trim(sc.Bytes(), " \t\r")
		if len(line) == 0 {
			continue
		}
		lines++
		e, ok := parseLine(line)
		if !ok {
			bad++
			continue
		}
		*lookup(methods, e.method)++
		status[e.status]++
		ps := lookup(paths, e.path)
		ps.Count++
		ps.TotalMillis += e.duration
		ps.MaxMillis = max(ps.MaxMillis, e.duration)
		ps.Bytes += e.bytes
	}

	s := Summary{
		Lines:     lines,
		Malformed: bad,
		Methods:   make(map[string]int, len(methods)),
		Status:    make(map[int]int),
		Paths:     make(map[string]PathStats, len(paths)),
	}
	for m, n := range methods {
		s.Methods[m] = *n
	}
	for code, n := range status {
		if n > 0 {
			s.Status[code] = n
		}
	}
	for p, ps := range paths {
		s.Paths[p] = *ps
	}
	if err := sc.Err(); err != nil {
		return s, fmt.Errorf("logstats: read: %w", err)
	}
	return s, nil
}

// lookup returns the entry for key in m, creating it if needed.
//
// The read m[string(key)] does not allocate (the compiler recognises the
// pattern), whereas an assignment m[string(key)] = v always allocates a new
// string. Storing pointers means we assign only once per distinct key.
func lookup[V any](m map[string]*V, key []byte) *V {
	if p, ok := m[string(key)]; ok {
		return p
	}
	p := new(V)
	m[string(key)] = p
	return p
}

// parseLine parses and validates one non-blank, trimmed line. It reports
// false for a malformed line; it deliberately returns a bool rather than an
// error, because building an error (fmt.Errorf) allocates and malformed
// lines are not exceptional in a log file.
func parseLine(line []byte) (entry, bool) {
	var f [6][]byte
	rest := line
	for i := range f {
		f[i], rest = nextField(rest)
		if len(f[i]) == 0 {
			return entry{}, false // fewer than 6 fields
		}
	}
	if len(rest) != 0 {
		return entry{}, false // more than 6 fields (line is trimmed, so rest is a field)
	}
	method, path := f[1], f[2]
	for _, c := range method {
		if c < 'A' || c > 'Z' {
			return entry{}, false
		}
	}
	if path[0] != '/' {
		return entry{}, false
	}
	status, ok := parseUint(f[3])
	if !ok || len(f[3]) != 3 || status < 100 || status > 599 {
		return entry{}, false
	}
	dur, ok := parseUint(f[4])
	if !ok {
		return entry{}, false
	}
	size, ok := parseUint(f[5])
	if !ok {
		return entry{}, false
	}
	return entry{method: method, path: path, status: int(status), duration: dur, bytes: size}, true
}

// nextField skips leading separators in b and returns the next field and
// whatever follows it.
func nextField(b []byte) (field, rest []byte) {
	i := 0
	for i < len(b) && isSep(b[i]) {
		i++
	}
	b = b[i:]
	j := 0
	for j < len(b) && !isSep(b[j]) {
		j++
	}
	return b[:j], b[j:]
}

func isSep(c byte) bool { return c == ' ' || c == '\t' }

// parseUint parses a non-empty run of ASCII digits that fits in an int64.
// Unlike strconv.ParseInt it takes []byte (no string conversion) and rejects
// signs.
func parseUint(b []byte) (int64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	var n int64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		d := int64(c - '0')
		if n > (math.MaxInt64-d)/10 {
			return 0, false // overflow
		}
		n = n*10 + d
	}
	return n, true
}
