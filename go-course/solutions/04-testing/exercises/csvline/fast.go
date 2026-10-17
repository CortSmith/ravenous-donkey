package csvline

import (
	"fmt"
	"strings"
)

// JoinFast returns the same line as Join with a single allocation: it sizes
// the output up front and writes doubled quotes byte by byte instead of
// building a temporary string with strings.ReplaceAll for every quoted
// field.
func JoinFast(fields []string) string {
	n := max(len(fields)-1, 0) // commas
	for _, f := range fields {
		n += len(f)
		if needsQuotes(f) {
			n += 2 + strings.Count(f, `"`)
		}
	}
	var b strings.Builder
	b.Grow(n)
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		if !needsQuotes(f) {
			b.WriteString(f)
			continue
		}
		b.WriteByte('"')
		for j := 0; j < len(f); j++ {
			if f[j] == '"' {
				b.WriteByte('"')
			}
			b.WriteByte(f[j])
		}
		b.WriteByte('"')
	}
	return b.String()
}

func needsQuotes(f string) bool {
	return strings.ContainsAny(f, ",\"\r\n")
}

// SplitFast returns the same fields and errors as Split. It allocates the
// result slice once (sized by counting commas, an upper bound on the field
// count) and returns quoted fields that contain no escaped quote as
// substrings of line instead of copying them through a strings.Builder.
func SplitFast(line string) ([]string, error) {
	fields := make([]string, 0, strings.Count(line, ",")+1)
	i := 0
	for {
		var field string
		if i < len(line) && line[i] == '"' {
			i++ // opening quote
			start := i
			escaped := false
			for {
				j := strings.IndexByte(line[i:], '"')
				if j < 0 {
					return nil, ErrUnterminated
				}
				i += j + 1
				if i < len(line) && line[i] == '"' {
					escaped = true
					i++
					continue
				}
				break
			}
			field = line[start : i-1]
			if escaped {
				field = strings.ReplaceAll(field, `""`, `"`)
			}
			if i < len(line) && line[i] != ',' {
				return nil, fmt.Errorf("%w at byte %d near %q", ErrSyntax, i, line[i:min(i+8, len(line))])
			}
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
