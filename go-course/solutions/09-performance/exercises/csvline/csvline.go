// Package csvline encodes a single CSV record into a string.
package csvline

import "strings"

// Line encodes fields as one CSV record (RFC 4180 style) without a trailing
// newline. A field is wrapped in double quotes when it contains a comma, a
// double quote, a carriage return or a line feed; inside a quoted field every
// double quote is doubled. An empty fields slice yields "".
//
// Line performs at most one heap allocation: the exact output size is computed
// first so the strings.Builder never has to grow.
func Line(fields []string) string {
	var b strings.Builder
	b.Grow(encodedLen(fields))
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		if !needsQuotes(f) {
			b.WriteString(f)
			continue
		}
		b.WriteByte('"')
		for {
			j := strings.IndexByte(f, '"')
			if j < 0 {
				break
			}
			b.WriteString(f[:j+1]) // includes the quote...
			b.WriteByte('"')       // ...which we then double
			f = f[j+1:]
		}
		b.WriteString(f)
		b.WriteByte('"')
	}
	return b.String()
}

// needsQuotes reports whether f must be quoted in a CSV record.
func needsQuotes(f string) bool {
	return strings.ContainsAny(f, ",\"\r\n")
}

// encodedLen returns the exact length of Line(fields).
func encodedLen(fields []string) int {
	if len(fields) == 0 {
		return 0
	}
	n := len(fields) - 1 // separators
	for _, f := range fields {
		n += len(f)
		if needsQuotes(f) {
			n += 2 + strings.Count(f, `"`)
		}
	}
	return n
}
