package closeerr

import (
	"errors"
	"fmt"
	"testing"
)

func TestWriteLinesFastMatches(t *testing.T) {
	inputs := [][]string{nil, {"a"}, {"a", "", "ccc"}, {"héllo", "世界"}}
	for _, lines := range inputs {
		slow, fast := &fakeFile{}, &fakeFile{}
		errS, errF := WriteLines(slow, lines), WriteLinesFast(fast, lines)
		if errS != nil || errF != nil {
			t.Fatalf("unexpected errors: %v, %v", errS, errF)
		}
		if slow.buf.String() != fast.buf.String() {
			t.Errorf("WriteLinesFast(%q) wrote %q, want %q", lines, fast.buf.String(), slow.buf.String())
		}
		if fast.closeCalls != 1 {
			t.Errorf("Close called %d times, want 1", fast.closeCalls)
		}
	}
	f := &fakeFile{failWrite: true, closeErr: errClose}
	err := WriteLinesFast(f, []string{"x"})
	if !errors.Is(err, errWrite) || !errors.Is(err, errClose) {
		t.Errorf("WriteLinesFast with both failures = %v, want both errors", err)
	}
}

// discardCloser is a cheap io.WriteCloser for benchmarks.
type discardCloser struct{}

func (discardCloser) Write(p []byte) (int, error) { return len(p), nil }
func (discardCloser) Close() error                { return nil }

func benchLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line number %d with some text", i)
	}
	return lines
}

func BenchmarkWriteLines10(b *testing.B) {
	lines := benchLines(10)
	b.ReportAllocs()
	for b.Loop() {
		_ = WriteLines(discardCloser{}, lines)
	}
}

func BenchmarkWriteLinesFast10(b *testing.B) {
	lines := benchLines(10)
	b.ReportAllocs()
	for b.Loop() {
		_ = WriteLinesFast(discardCloser{}, lines)
	}
}

func BenchmarkWriteLines1000(b *testing.B) {
	lines := benchLines(1000)
	b.ReportAllocs()
	for b.Loop() {
		_ = WriteLines(discardCloser{}, lines)
	}
}

func BenchmarkWriteLinesFast1000(b *testing.B) {
	lines := benchLines(1000)
	b.ReportAllocs()
	for b.Loop() {
		_ = WriteLinesFast(discardCloser{}, lines)
	}
}
