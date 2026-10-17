package closeerr

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	errWrite = errors.New("device unplugged")
	errClose = errors.New("disk full")
)

// fakeFile is an io.WriteCloser whose failures we control.
type fakeFile struct {
	buf        bytes.Buffer
	failWrite  bool
	closeErr   error
	closeCalls int
	writeCalls int
}

func (f *fakeFile) Write(p []byte) (int, error) {
	f.writeCalls++
	if f.closeCalls > 0 {
		return 0, errors.New("write after close")
	}
	if f.failWrite {
		return 0, errWrite
	}
	return f.buf.Write(p)
}

func (f *fakeFile) Close() error {
	f.closeCalls++
	return f.closeErr
}

func TestWriteLinesSuccess(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"nil slice", nil, ""},
		{"one line", []string{"hello"}, "hello\n"},
		{"several lines", []string{"a", "", "c"}, "a\n\nc\n"},
		{"unicode", []string{"héllo", "世界"}, "héllo\n世界\n"},
		{"bigger than bufio buffer", []string{strings.Repeat("x", 10000)}, strings.Repeat("x", 10000) + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeFile{}
			if err := WriteLines(f, tt.lines); err != nil {
				t.Fatalf("WriteLines(%q) unexpected error: %v", tt.lines, err)
			}
			if got := f.buf.String(); got != tt.want {
				t.Errorf("WriteLines(%q) wrote %q, want %q (did you Flush?)", tt.lines, got, tt.want)
			}
			if f.closeCalls != 1 {
				t.Errorf("Close called %d times, want exactly 1", f.closeCalls)
			}
		})
	}
}

func TestWriteLinesErrors(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string
		failWrite   bool
		closeErr    error
		wantWrite   bool   // errors.Is(err, errWrite)
		wantClose   bool   // errors.Is(err, errClose)
		wantMessage string // exact, if non-empty
	}{
		{
			name:        "close fails after successful write",
			lines:       []string{"a", "b"},
			closeErr:    errClose,
			wantClose:   true,
			wantMessage: "close: disk full",
		},
		{
			name:        "close fails with nothing written",
			lines:       nil,
			closeErr:    errClose,
			wantClose:   true,
			wantMessage: "close: disk full",
		},
		{
			name:      "write fails (surfaces at flush)",
			lines:     []string{"a"},
			failWrite: true,
			wantWrite: true,
		},
		{
			name:      "write fails on large input",
			lines:     []string{strings.Repeat("y", 10000)},
			failWrite: true,
			wantWrite: true,
		},
		{
			name:      "write and close both fail",
			lines:     []string{"a"},
			failWrite: true,
			closeErr:  errClose,
			wantWrite: true,
			wantClose: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeFile{failWrite: tt.failWrite, closeErr: tt.closeErr}
			err := WriteLines(f, tt.lines)
			if err == nil {
				t.Fatalf("WriteLines(...) = nil, want error (write fails: %v, close error: %v)", tt.failWrite, tt.closeErr)
			}
			if got := errors.Is(err, errWrite); got != tt.wantWrite {
				t.Errorf("errors.Is(err, errWrite) = %v, want %v; err = %q", got, tt.wantWrite, err)
			}
			if got := errors.Is(err, errClose); got != tt.wantClose {
				t.Errorf("errors.Is(err, errClose) = %v, want %v; err = %q", got, tt.wantClose, err)
			}
			if tt.wantMessage != "" && err.Error() != tt.wantMessage {
				t.Errorf("err.Error() = %q, want %q", err.Error(), tt.wantMessage)
			}
			if err.Error() == errWrite.Error() {
				t.Errorf("write error returned without context: %q", err)
			}
			if f.closeCalls != 1 {
				t.Errorf("Close called %d times, want exactly 1 even on failure", f.closeCalls)
			}
		})
	}
}

func TestSaveFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	if err := SaveFile(path, []string{"one", "two"}); err != nil {
		t.Fatalf("SaveFile(%q) unexpected error: %v", path, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "one\ntwo\n"; string(got) != want {
		t.Errorf("file contents = %q, want %q", got, want)
	}

	// Overwrites rather than appends.
	if err := SaveFile(path, []string{"three"}); err != nil {
		t.Fatalf("second SaveFile unexpected error: %v", err)
	}
	got, _ = os.ReadFile(path)
	if want := "three\n"; string(got) != want {
		t.Errorf("after overwrite, file contents = %q, want %q", got, want)
	}
}

func TestSaveFileCreateError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no", "such", "dir", "out.txt")
	err := SaveFile(path, []string{"x"})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("SaveFile(%q) = %v, want error matching fs.ErrNotExist", path, err)
	}
	var pe *fs.PathError
	if !errors.As(err, &pe) {
		t.Errorf("SaveFile error %v (%T) should be (or wrap) *fs.PathError", err, err)
	}
	if strings.Count(err.Error(), path) != 1 {
		t.Errorf("err = %q: path should appear exactly once (PathError already includes it)", err)
	}
}
