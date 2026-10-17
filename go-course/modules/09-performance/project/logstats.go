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
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
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

// entry is one parsed log line.
type entry struct {
	method   string
	path     string
	status   int
	duration int64
	bytes    int64
}

var (
	methodRE = regexp.MustCompile(`^[A-Z]+$`)
	statusRE = regexp.MustCompile(`^[1-5][0-9][0-9]$`)
	numberRE = regexp.MustCompile(`^[0-9]+$`)
)

// Summarize reads an access log from r and aggregates it.
//
// Leading and trailing spaces, tabs and carriage returns are ignored, and
// lines that are then empty are skipped and not counted. Only ' ' and '\t'
// separate fields (other whitespace is part of a field). A non-blank line is well-formed when it has exactly 6 fields, the method is
// one or more ASCII upper-case letters, the path starts with '/', the status
// is three digits in 100-599, and duration and bytes are unsigned decimal
// integers that fit in an int64. Any other non-blank line counts as
// Malformed and is otherwise ignored.
//
// The maps in the returned Summary are never nil. Summarize returns an error
// only if reading fails (including a line longer than bufio.MaxScanTokenSize);
// the Summary of the lines read so far is returned with it.
//
// This implementation is CORRECT but SLOW. Your job is to make it fast; see
// README.md.
func Summarize(r io.Reader) (Summary, error) {
	s := Summary{
		Methods: map[string]int{},
		Status:  map[int]int{},
		Paths:   map[string]PathStats{},
	}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.Trim(sc.Text(), " \t\r")
		if line == "" {
			continue
		}
		s.Lines++
		e, err := parseLine(line)
		if err != nil {
			s.Malformed++
			continue
		}
		s.Methods[e.method]++
		s.Status[e.status]++
		ps := s.Paths[e.path]
		ps.Count++
		ps.TotalMillis += e.duration
		ps.MaxMillis = max(ps.MaxMillis, e.duration)
		ps.Bytes += e.bytes
		s.Paths[e.path] = ps
	}
	if err := sc.Err(); err != nil {
		return s, fmt.Errorf("logstats: read: %w", err)
	}
	return s, nil
}

// isSep reports whether r separates fields.
func isSep(r rune) bool { return r == ' ' || r == '\t' }

// parseLine parses and validates one non-blank, trimmed line.
func parseLine(line string) (entry, error) {
	f := strings.FieldsFunc(line, isSep)
	if len(f) != 6 {
		return entry{}, fmt.Errorf("want 6 fields, got %d", len(f))
	}
	if !methodRE.MatchString(f[1]) {
		return entry{}, fmt.Errorf("bad method %q", f[1])
	}
	if !strings.HasPrefix(f[2], "/") {
		return entry{}, fmt.Errorf("bad path %q", f[2])
	}
	if !statusRE.MatchString(f[3]) {
		return entry{}, fmt.Errorf("bad status %q", f[3])
	}
	status, err := strconv.Atoi(f[3])
	if err != nil {
		return entry{}, fmt.Errorf("bad status %q: %w", f[3], err)
	}
	if !numberRE.MatchString(f[4]) || !numberRE.MatchString(f[5]) {
		return entry{}, fmt.Errorf("bad number in %q", line)
	}
	dur, err := strconv.ParseInt(f[4], 10, 64)
	if err != nil {
		return entry{}, fmt.Errorf("bad duration: %w", err)
	}
	size, err := strconv.ParseInt(f[5], 10, 64)
	if err != nil {
		return entry{}, fmt.Errorf("bad bytes: %w", err)
	}
	return entry{method: f[1], path: f[2], status: status, duration: dur, bytes: size}, nil
}
