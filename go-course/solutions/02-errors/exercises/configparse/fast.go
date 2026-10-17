package configparse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// ParseFast behaves exactly like Parse but allocates less on the happy path.
//
// In Parse, sc.Text() allocates one string per line, comments included.
// Key and value are substrings of that line, so they share its memory and
// cost nothing extra. ParseFast keeps that sharing but skips the comment
// lines:
//
//   - it inspects sc.Bytes() (no allocation) to skip blank and comment lines;
//   - only for a real "key=value" line does it convert the trimmed line to a
//     string, once, and slice key and value out of it.
//
// For a typical file where half the lines are comments, that halves the
// per-line allocations. Error construction is unchanged: errors are rare, so
// they may stay slow and readable.
func ParseFast(r io.Reader) (map[string]string, error) {
	cfg := make(map[string]string)
	firstSeen := make(map[string]int)

	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 || b[0] == '#' {
			continue
		}
		line := string(b) // the only allocation for this line
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, &ParseError{Line: lineNo, Err: ErrMissingEquals}
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, &ParseError{Line: lineNo, Err: ErrEmptyKey}
		}
		if !validKey(key) {
			return nil, &ParseError{Line: lineNo, Key: key, Err: ErrInvalidKey}
		}
		if first, dup := firstSeen[key]; dup {
			return nil, &ParseError{
				Line: lineNo,
				Key:  key,
				Err:  fmt.Errorf("%w (first defined on line %d)", ErrDuplicateKey, first),
			}
		}
		firstSeen[key] = lineNo
		cfg[key] = value
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return cfg, nil
}
