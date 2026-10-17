// Package configparse parses a tiny key=value configuration format and
// reports problems as structured *ParseError values.
//
// Format rules:
//   - One "key = value" pair per line. Whitespace around key and value is trimmed.
//   - Blank lines and lines whose first non-space character is '#' are ignored.
//   - Everything after the FIRST '=' is the value, so "url=a=b" has value "a=b".
//   - Values may be empty ("debug=" is valid).
//   - Keys must be non-empty and use only ASCII letters, digits, '_', '.', '-'.
//   - A key may appear only once.
package configparse

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Sentinel errors describing what is wrong with a line. Callers match them
// with errors.Is; they are always delivered wrapped inside a *ParseError.
var (
	ErrMissingEquals = errors.New("missing '='")
	ErrEmptyKey      = errors.New("empty key")
	ErrInvalidKey    = errors.New("invalid key")
	ErrDuplicateKey  = errors.New("duplicate key")
)

// ParseError reports a problem on a specific line of the input.
type ParseError struct {
	Line int    // 1-based line number
	Key  string // offending key, or "" when no key could be extracted
	Err  error  // underlying cause; wraps one of the Err* sentinels
}

// Error formats the error as "line N: cause" or, when a key is known,
// "line N: key \"k\": cause".
func (e *ParseError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("line %d: key %q: %v", e.Line, e.Key, e.Err)
	}
	return fmt.Sprintf("line %d: %v", e.Line, e.Err)
}

// Unwrap returns the underlying cause so errors.Is/As can see through it.
func (e *ParseError) Unwrap() error { return e.Err }

// Parse reads configuration from r. It stops at the first malformed line and
// returns a *ParseError for it. Errors from r itself are returned wrapped
// (not as *ParseError) so that callers can still match them with errors.Is.
// On any error the returned map is nil.
func Parse(r io.Reader) (map[string]string, error) {
	cfg := make(map[string]string)
	firstSeen := make(map[string]int) // key -> line it was defined on

	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

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

// validKey reports whether every byte of key is an allowed ASCII character.
func validKey(key string) bool {
	for i := 0; i < len(key); i++ {
		c := key[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case c == '_', c == '.', c == '-':
		default:
			return false
		}
	}
	return true
}
