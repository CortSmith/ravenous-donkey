package logging

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
)

var (
	_ io.Writer = (*CountingWriter)(nil)
	_ Store     = (*AuditedStore)(nil)
)

// recordingWriter remembers every Write call separately.
type recordingWriter struct{ calls []string }

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.calls = append(w.calls, string(p))
	return len(p), nil
}

func TestLogf(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		format string
		args   []any
		want   string
	}{
		{"plain", "", "hello", nil, "hello\n"},
		{"prefix", "[app] ", "started", nil, "[app] started\n"},
		{"formatting", "db: ", "%d rows in %s", []any{3, "users"}, "db: 3 rows in users\n"},
		{"no double newline", "> ", "done\n", nil, "> done\n"},
		{"empty message", "", "", nil, "\n"},
		{"percent in args is not reinterpreted", "", "%s", []any{"100%"}, "100%\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &recordingWriter{}
			l := &Logger{Prefix: tt.prefix, Out: w}
			l.Logf(tt.format, tt.args...)
			if len(w.calls) != 1 {
				t.Fatalf("Logf(%q) made %d Write calls %q, want exactly 1 (build the whole line first)", tt.format, len(w.calls), w.calls)
			}
			if w.calls[0] != tt.want {
				t.Errorf("Logf(%q, %v) wrote %q, want %q", tt.format, tt.args, w.calls[0], tt.want)
			}
		})
	}
}

func TestLogfNilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Logf panicked: %v (a nil *Logger and a nil Out must be no-ops)", r)
		}
	}()
	var l *Logger
	l.Logf("ignored %d", 1)
	(&Logger{Prefix: "x"}).Logf("also ignored")
}

func TestCountingWriter(t *testing.T) {
	var sb strings.Builder
	cw := &CountingWriter{Writer: &sb}

	// CountingWriter must work anywhere an io.Writer is accepted.
	fmt.Fprintf(cw, "line one\nline two\n")
	io.WriteString(cw, "no newline")
	cw.Write(nil)

	if got, want := sb.String(), "line one\nline two\nno newline"; got != want {
		t.Errorf("underlying writer got %q, want %q", got, want)
	}
	if got, want := cw.Bytes, int64(len("line one\nline two\nno newline")); got != want {
		t.Errorf("Bytes = %d, want %d", got, want)
	}
	if got, want := cw.Lines, 2; got != want {
		t.Errorf("Lines = %d, want %d", got, want)
	}
}

// shortWriter accepts at most limit bytes in total, then fails.
type shortWriter struct {
	limit int
	buf   []byte
}

var errFull = errors.New("full")

func (w *shortWriter) Write(p []byte) (int, error) {
	room := w.limit - len(w.buf)
	if len(p) <= room {
		w.buf = append(w.buf, p...)
		return len(p), nil
	}
	w.buf = append(w.buf, p[:room]...)
	return room, errFull
}

func TestCountingWriterPartialWrite(t *testing.T) {
	sw := &shortWriter{limit: 5}
	cw := &CountingWriter{Writer: sw}
	n, err := cw.Write([]byte("ab\ncd\nef\n"))
	if n != 5 || !errors.Is(err, errFull) {
		t.Fatalf("Write = %d, %v; want 5, errFull (return the underlying writer's results unchanged)", n, err)
	}
	if cw.Bytes != 5 {
		t.Errorf("Bytes = %d after partial write, want 5 (count only bytes actually written)", cw.Bytes)
	}
	if cw.Lines != 1 {
		t.Errorf("Lines = %d after partial write of %q, want 1", cw.Lines, sw.buf)
	}
}

// fakeStore records which methods were called, so we can check that
// AuditedStore delegates to the store it wraps.
type fakeStore struct {
	MapStore
	calls []string
}

func (f *fakeStore) Get(key string) (string, bool) {
	f.calls = append(f.calls, "Get "+key)
	return f.MapStore.Get(key)
}

func (f *fakeStore) Set(key, value string) {
	f.calls = append(f.calls, "Set "+key)
	f.MapStore.Set(key, value)
}

func (f *fakeStore) Delete(key string) {
	f.calls = append(f.calls, "Delete "+key)
	f.MapStore.Delete(key)
}

func TestAuditedStore(t *testing.T) {
	inner := &fakeStore{}
	logOut := &recordingWriter{}
	s := NewAuditedStore(inner, &Logger{Prefix: "audit: ", Out: logOut})

	s.Set("a", "1")
	s.Set("b", "2")
	if v, ok := s.Get("a"); !ok || v != "1" {
		t.Errorf(`Get("a") = %q, %v; want "1", true`, v, ok)
	}
	s.Delete("a")
	if v, ok := s.Get("a"); ok {
		t.Errorf(`Get("a") after Delete = %q, true; want "", false`, v)
	}

	wantLog := []string{"audit: set a=1\n", "audit: set b=2\n", "audit: delete a\n"}
	if !slices.Equal(logOut.calls, wantLog) {
		t.Errorf("audit log = %q, want %q (Get must not be logged)", logOut.calls, wantLog)
	}
	wantCalls := []string{"Set a", "Set b", "Get a", "Delete a", "Get a"}
	if !slices.Equal(inner.calls, wantCalls) {
		t.Errorf("calls reaching the wrapped store = %q, want %q", inner.calls, wantCalls)
	}
}

func TestAuditedStoreWithoutLogger(t *testing.T) {
	s := NewAuditedStore(&MapStore{}, nil)
	s.Set("k", "v") // must not panic with a nil *Logger
	s.Delete("missing")
	if v, ok := s.Get("k"); !ok || v != "v" {
		t.Errorf(`Get("k") = %q, %v; want "v", true`, v, ok)
	}
}

func TestAuditedStoreIsAStore(t *testing.T) {
	// Code that only knows about the Store interface still gets auditing,
	// because the overriding methods are in *AuditedStore's method set.
	logOut := &recordingWriter{}
	var s Store = NewAuditedStore(&MapStore{}, &Logger{Out: logOut})
	s.Set("x", "y")
	if len(logOut.calls) != 1 || logOut.calls[0] != "set x=y\n" {
		t.Errorf("Set through the Store interface logged %q, want [\"set x=y\\n\"]", logOut.calls)
	}
}
