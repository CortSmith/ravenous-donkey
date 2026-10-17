package configparse

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"
	"testing"
	"testing/iotest"
)

func TestParseValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{"empty input", "", map[string]string{}},
		{"only comments and blanks", "# hello\n\n   \n  # indented comment\n", map[string]string{}},
		{"single pair", "host=localhost", map[string]string{"host": "localhost"}},
		{"trims whitespace", "  host  =   example.com  \n", map[string]string{"host": "example.com"}},
		{"empty value allowed", "debug=", map[string]string{"debug": ""}},
		{"value keeps later equals", "dsn=user=bob;pw=x", map[string]string{"dsn": "user=bob;pw=x"}},
		{"value keeps hash", "color = #ff0000", map[string]string{"color": "#ff0000"}},
		{"allowed key characters", "db.max-conns_2 = 10", map[string]string{"db.max-conns_2": "10"}},
		{"CRLF line endings", "a=1\r\nb=2\r\n", map[string]string{"a": "1", "b": "2"}},
		{"no trailing newline", "a=1\nb=2", map[string]string{"a": "1", "b": "2"}},
		{"tabs around key", "\tport\t=\t8080", map[string]string{"port": "8080"}},
		{
			"realistic file",
			"# server\nhost = 0.0.0.0\nport = 8080\n\n# features\nbeta=true\n",
			map[string]string{"host": "0.0.0.0", "port": "8080", "beta": "true"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", tt.input, err)
			}
			if got == nil {
				t.Fatalf("Parse(%q) returned a nil map with nil error; want an empty, non-nil map", tt.input)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLine int
		wantKey  string
		wantIs   error
		wantMsg  string
	}{
		{
			name:     "missing equals",
			input:    "host=a\nport 8080\n",
			wantLine: 2,
			wantIs:   ErrMissingEquals,
			wantMsg:  "line 2: missing '='",
		},
		{
			name:     "empty key",
			input:    "# c\n\n = value\n",
			wantLine: 3,
			wantIs:   ErrEmptyKey,
			wantMsg:  "line 3: empty key",
		},
		{
			name:     "key with space",
			input:    "my key = 1",
			wantLine: 1,
			wantKey:  "my key",
			wantIs:   ErrInvalidKey,
			wantMsg:  `line 1: key "my key": invalid key`,
		},
		{
			name:     "non-ascii key",
			input:    "a=1\ncafé=2",
			wantLine: 2,
			wantKey:  "café",
			wantIs:   ErrInvalidKey,
			wantMsg:  `line 2: key "café": invalid key`,
		},
		{
			name:     "duplicate key",
			input:    "a=1\n# comment\nb=2\na = 3\n",
			wantLine: 4,
			wantKey:  "a",
			wantIs:   ErrDuplicateKey,
			wantMsg:  `line 4: key "a": duplicate key (first defined on line 1)`,
		},
		{
			name:     "first error wins",
			input:    "ok=1\nbad line\n=also bad\n",
			wantLine: 2,
			wantIs:   ErrMissingEquals,
			wantMsg:  "line 2: missing '='",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(tt.input))
			if err == nil {
				t.Fatalf("Parse(%q) = %v, nil; want an error", tt.input, got)
			}
			if got != nil {
				t.Errorf("Parse(%q) returned non-nil map %v alongside an error; want nil", tt.input, got)
			}

			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Parse(%q) error %v (type %T) is not a *ParseError", tt.input, err, err)
			}
			if pe.Line != tt.wantLine {
				t.Errorf("ParseError.Line = %d, want %d", pe.Line, tt.wantLine)
			}
			if pe.Key != tt.wantKey {
				t.Errorf("ParseError.Key = %q, want %q", pe.Key, tt.wantKey)
			}
			if !errors.Is(err, tt.wantIs) {
				t.Errorf("errors.Is(err, %v) = false; err = %v. Does ParseError implement Unwrap?", tt.wantIs, err)
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("err.Error() = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

// The caller may wrap our error again; errors.As must still find it.
func TestParseErrorSurvivesWrapping(t *testing.T) {
	_, err := Parse(strings.NewReader("oops"))
	wrapped := fmt.Errorf("load app.conf: %w", err)

	var pe *ParseError
	if !errors.As(wrapped, &pe) {
		t.Fatalf("errors.As(wrapped, *ParseError) = false for %v", wrapped)
	}
	if pe.Line != 1 {
		t.Errorf("Line = %d, want 1", pe.Line)
	}
	if !errors.Is(wrapped, ErrMissingEquals) {
		t.Errorf("errors.Is(wrapped, ErrMissingEquals) = false for %v", wrapped)
	}
}

func TestParseErrorMethods(t *testing.T) {
	cause := errors.New("boom")
	tests := []struct {
		in   *ParseError
		want string
	}{
		{&ParseError{Line: 7, Err: cause}, "line 7: boom"},
		{&ParseError{Line: 1, Key: "k", Err: cause}, `line 1: key "k": boom`},
	}
	for _, tt := range tests {
		if got := tt.in.Error(); got != tt.want {
			t.Errorf("(%#v).Error() = %q, want %q", *tt.in, got, tt.want)
		}
		if got := tt.in.Unwrap(); got != cause {
			t.Errorf("(%#v).Unwrap() = %v, want %v", *tt.in, got, cause)
		}
	}
}

// I/O failures are not syntax errors: they must be wrapped with %w, keep the
// original error matchable, and must NOT be reported as *ParseError.
func TestParseReaderError(t *testing.T) {
	errDisk := errors.New("disk on fire")
	r := io.MultiReader(strings.NewReader("a=1\nb=2\n"), iotest.ErrReader(errDisk))

	got, err := Parse(r)
	if err == nil {
		t.Fatalf("Parse(failing reader) = %v, nil; want error", got)
	}
	if got != nil {
		t.Errorf("Parse(failing reader) returned map %v alongside error; want nil", got)
	}
	if !errors.Is(err, errDisk) {
		t.Errorf("errors.Is(err, errDisk) = false; err = %v. Wrap with %%w, not %%v", err)
	}
	var pe *ParseError
	if errors.As(err, &pe) {
		t.Errorf("reader failure reported as *ParseError (%v); want a plain wrapped error", pe)
	}
	if err.Error() == errDisk.Error() {
		t.Errorf("err.Error() = %q: add context when wrapping (e.g. \"read config: ...\")", err.Error())
	}
}

// A line longer than bufio.Scanner's default limit (64 KiB) makes Scan stop.
// If you forget to check sc.Err(), this input silently "parses" as empty.
func TestParseLineTooLong(t *testing.T) {
	input := "a=1\nhuge=" + strings.Repeat("x", bufio.MaxScanTokenSize) + "\n"
	got, err := Parse(strings.NewReader(input))
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("Parse(70KB line) = %v, %v; want error wrapping bufio.ErrTooLong (did you check sc.Err()?)", len(got), err)
	}
}
