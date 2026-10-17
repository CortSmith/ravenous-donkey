// Package csvline encodes a single CSV record into a string.
package csvline

import "strings"

// Line encodes fields as one CSV record (RFC 4180 style) without a trailing
// newline. A field is wrapped in double quotes when it contains a comma, a
// double quote, a carriage return or a line feed; inside a quoted field every
// double quote is doubled. An empty fields slice yields "".
//
// This version is CORRECT but SLOW: every += copies the whole string built so
// far, so it allocates once (or more) per field and is O(n²) in output size.
//
// TODO: keep the behaviour, but make Line allocate at most once per call.
// Measure first: go test -run='^$' -bench=Line -benchmem ./modules/09-performance/exercises/csvline/
func Line(fields []string) string {
	s := ""
	for i, f := range fields {
		if i > 0 {
			s += ","
		}
		if strings.ContainsAny(f, ",\"\r\n") {
			f = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
		}
		s += f
	}
	return s
}
