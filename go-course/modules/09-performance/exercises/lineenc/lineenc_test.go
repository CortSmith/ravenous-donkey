package lineenc

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		in   Record
		want string
	}{
		{"basic", Record{"alice", 42, true}, `name="alice" score=42 passed=true` + "\n"},
		{"zero value", Record{}, `name="" score=0 passed=false` + "\n"},
		{"negative score", Record{"bob", -7, false}, `name="bob" score=-7 passed=false` + "\n"},
		{"large score", Record{"c", 1 << 40, true}, `name="c" score=1099511627776 passed=true` + "\n"},
		{"spaces and equals", Record{"a b=c", 1, true}, `name="a b=c" score=1 passed=true` + "\n"},
		{"quotes and backslash", Record{`say "hi" \o/`, 1, true}, `name="say \"hi\" \\o/" score=1 passed=true` + "\n"},
		{"newline and tab", Record{"x\ny\tz", 1, true}, `name="x\ny\tz" score=1 passed=true` + "\n"},
		{"unicode", Record{"héllo, 世界", 3, false}, `name="héllo, 世界" score=3 passed=false` + "\n"},
		{"invalid utf8", Record{"\xff", 3, false}, `name="\xff" score=3 passed=false` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewEncoder(&buf).Encode(tt.in); err != nil {
				t.Fatalf("Encode(%+v) returned error: %v", tt.in, err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("Encode(%+v) wrote %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestEncodeSequence catches buffer-reuse bugs: a long record followed by
// short ones must not leave stale bytes behind, and each record must be one
// Write call.
func TestEncodeSequence(t *testing.T) {
	w := &recordingWriter{}
	enc := NewEncoder(w)
	recs := []Record{
		{strings.Repeat("long", 100), 123456789, true},
		{"a", 1, false},
		{"", 0, true},
		{"bb", 22, false},
	}
	for _, r := range recs {
		if err := enc.Encode(r); err != nil {
			t.Fatalf("Encode(%+v) returned error: %v", r, err)
		}
	}
	want := []string{
		`name="` + strings.Repeat("long", 100) + `" score=123456789 passed=true` + "\n",
		`name="a" score=1 passed=false` + "\n",
		`name="" score=0 passed=true` + "\n",
		`name="bb" score=22 passed=false` + "\n",
	}
	if len(w.writes) != len(want) {
		t.Fatalf("got %d Write calls, want %d (one per record)", len(w.writes), len(want))
	}
	for i := range want {
		if w.writes[i] != want[i] {
			t.Errorf("Write #%d = %q, want %q", i, w.writes[i], want[i])
		}
	}
}

func TestEncodeWriteError(t *testing.T) {
	errDisk := errors.New("disk full")
	enc := NewEncoder(failingWriter{errDisk})
	err := enc.Encode(Record{"a", 1, true})
	if err == nil {
		t.Fatal("Encode with a failing writer returned nil error")
	}
	if !errors.Is(err, errDisk) {
		t.Errorf("Encode error = %v, want it to wrap %v", err, errDisk)
	}
}

func TestEncodeAllocs(t *testing.T) {
	enc := NewEncoder(io.Discard)
	recs := []Record{
		{"alice", 1234567, true},
		{"bob the builder", -98765, false},
		{`quoted "name"`, 4242, true},
	}
	for _, r := range recs { // warm up: let any internal buffer reach its final size
		if err := enc.Encode(r); err != nil {
			t.Fatalf("Encode(%+v) returned error: %v", r, err)
		}
	}
	allocs := testing.AllocsPerRun(100, func() {
		for _, r := range recs {
			if err := enc.Encode(r); err != nil {
				t.Fatalf("Encode(%+v) returned error: %v", r, err)
			}
		}
	})
	if allocs != 0 {
		t.Errorf("Encode of 3 records (after warm-up) allocates %.0f times, want 0.\n"+
			"Hint: keep a []byte in the Encoder, reset it with buf = buf[:0], and fill it "+
			"with strconv.AppendQuote / AppendInt / AppendBool.", allocs)
	}
}

// recordingWriter copies every Write into a separate string.
type recordingWriter struct{ writes []string }

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.writes = append(w.writes, string(p))
	return len(p), nil
}

type failingWriter struct{ err error }

func (w failingWriter) Write(p []byte) (int, error) { return 0, w.err }

func BenchmarkEncode(b *testing.B) {
	enc := NewEncoder(io.Discard)
	r := Record{"bob the builder", 1234567, true}
	b.ReportAllocs()
	for b.Loop() {
		enc.Encode(r)
	}
}
