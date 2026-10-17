// Package csvline encodes a list of fields as one comma-separated line and
// decodes it again. It is a deliberately small subset of RFC 4180; real
// code should use encoding/csv.
package csvline

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrUnterminated means a quoted field has no closing quote.
	ErrUnterminated = errors.New("csvline: unterminated quoted field")

	// ErrSyntax means a closing quote is followed by something other than
	// a comma or the end of the line.
	ErrSyntax = errors.New("csvline: unexpected character after closing quote")
)

// Join encodes fields as a single line, separated by commas.
//
// A field is written inside double quotes, with every double quote in it
// doubled, if it contains a comma, a double quote, a carriage return or a
// newline. Other fields, including empty ones, are written as they are.
// Join(nil) returns "".
func Join(fields []string) string {
	var b strings.Builder
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		if !strings.ContainsAny(f, ",\"\r\n") {
			b.WriteString(f)
			continue
		}
		b.WriteByte('"')
		b.WriteString(strings.ReplaceAll(f, `"`, `""`))
		b.WriteByte('"')
	}
	return b.String()
}

// Split decodes a line produced by Join: for any non-empty fields,
// Split(Join(fields)) returns fields. Split("") returns []string{""}.
//
// A field that starts with a double quote is a quoted field: it runs to
// the next lone double quote ("" inside it stands for one "), which must be
// followed by a comma or the end of the line. Double quotes anywhere else
// in a field are kept as they are. Errors wrap ErrUnterminated or
// ErrSyntax.
func Split(line string) ([]string, error) {
	var fields []string
	i := 0
	for {
		var field string
		if i < len(line) && line[i] == '"' {
			var b strings.Builder
			i++ // opening quote
			for {
				j := strings.IndexByte(line[i:], '"')
				if j < 0 {
					return nil, ErrUnterminated
				}
				b.WriteString(line[i : i+j])
				i += j + 1
				if i < len(line) && line[i] == '"' { // "" is an escaped quote
					b.WriteByte('"')
					i++
					continue
				}
				break // closing quote
			}
			if i < len(line) && line[i] != ',' {
				return nil, fmt.Errorf("%w at byte %d near %q", ErrSyntax, i, line[i:min(i+8, len(line))])
			}
			field = b.String()
		} else {
			j := strings.IndexByte(line[i:], ',')
			if j < 0 {
				j = len(line) - i
			}
			field = line[i : i+j]
			i += j
		}
		fields = append(fields, field)
		if i == len(line) {
			return fields, nil
		}
		i++ // the comma
	}
}
