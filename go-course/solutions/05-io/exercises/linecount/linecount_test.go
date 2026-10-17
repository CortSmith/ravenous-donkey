package linecount

import (
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
)

func TestCount(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Stats
	}{
		{"empty", "", Stats{0, 0}},
		{"single newline", "\n", Stats{1, 0}},
		{"one line terminated", "hello\n", Stats{1, 5}},
		{"one line unterminated", "hello", Stats{1, 5}},
		{"two lines unterminated last", "a\nbcd", Stats{2, 3}},
		{"blank lines", "\n\n\n", Stats{3, 0}},
		{"longest in middle", "ab\nabcdef\nabc\n", Stats{3, 6}},
		{"crlf terminators", "ab\r\nabcd\r\n", Stats{2, 4}},
		{"crlf unterminated last", "abc\r\nxy", Stats{2, 3}},
		{"lone cr is content", "a\rb\n", Stats{1, 3}},
		{"trailing cr at eof is content", "abc\r", Stats{1, 4}},
		{"utf-8 counted in bytes", "日本語\n", Stats{1, 9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Count(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("Count(%q) error = %v, want nil", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Count(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

// TestLongLines uses lines far longer than bufio.MaxScanTokenSize (64 KiB).
// A plain bufio.Scanner stops with bufio.ErrTooLong on these.
func TestLongLines(t *testing.T) {
	long := strings.Repeat("x", 1<<20) // 1 MiB
	tests := []struct {
		name string
		in   string
		want Stats
	}{
		{"one huge line", long + "\n", Stats{1, 1 << 20}},
		{"huge unterminated", long, Stats{1, 1 << 20}},
		{"huge between short", "a\n" + long + "\nb\n", Stats{3, 1 << 20}},
		{"huge crlf", long + "\r\n", Stats{1, 1 << 20}},
		{"exactly 64KiB", strings.Repeat("y", 64*1024) + "\n", Stats{1, 64 * 1024}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Count(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("Count error = %v, want nil (did you hit bufio.Scanner's token limit?)", err)
			}
			if got != tt.want {
				t.Errorf("Count = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestCRLFAcrossBufferBoundaries feeds the input one byte at a time so that
// "\r" and "\n" always arrive in separate Read calls, and also places the
// "\r" at every position relative to typical buffer sizes.
func TestCRLFAcrossBufferBoundaries(t *testing.T) {
	for _, size := range []int{4095, 4096, 4097, 65535, 65536} {
		in := strings.Repeat("z", size) + "\r\n" + "tail\r\n"
		want := Stats{Lines: 2, Longest: size}
		got, err := Count(iotest.OneByteReader(strings.NewReader(in)))
		if err != nil {
			t.Fatalf("size %d: Count error = %v", size, err)
		}
		if got != want {
			t.Errorf("size %d: Count = %+v, want %+v", size, got, want)
		}
		got, err = Count(strings.NewReader(in))
		if err != nil {
			t.Fatalf("size %d: Count error = %v", size, err)
		}
		if got != want {
			t.Errorf("size %d (whole reader): Count = %+v, want %+v", size, got, want)
		}
	}
}

func TestDataWithEOF(t *testing.T) {
	got, err := Count(iotest.DataErrReader(strings.NewReader("a\nbb\nccc")))
	if err != nil {
		t.Fatalf("Count error = %v", err)
	}
	if want := (Stats{3, 3}); got != want {
		t.Errorf("Count = %+v, want %+v", got, want)
	}
}

func TestPropagatesErrors(t *testing.T) {
	boom := errors.New("disk on fire")
	r := io.MultiReader(strings.NewReader("one\ntwo\n"), iotest.ErrReader(boom))
	got, err := Count(r)
	if !errors.Is(err, boom) {
		t.Fatalf("Count error = %v, want %v", err, boom)
	}
	if want := (Stats{2, 3}); got != want {
		t.Errorf("Count stats before error = %+v, want %+v", got, want)
	}
}

// TestAgainstReference cross-checks Count against a simple strings.Split
// implementation on many generated inputs.
func TestAgainstReference(t *testing.T) {
	pieces := []string{"", "a", "bc", "\n", "\r", "\r\n", "def", "\n\n", "日"}
	// Enumerate all 3-piece combinations.
	for _, p1 := range pieces {
		for _, p2 := range pieces {
			for _, p3 := range pieces {
				in := p1 + p2 + p3
				want := reference(in)
				got, err := Count(strings.NewReader(in))
				if err != nil {
					t.Fatalf("Count(%q) error = %v", in, err)
				}
				if got != want {
					t.Errorf("Count(%q) = %+v, want %+v", in, got, want)
				}
			}
		}
	}
}

// TestBoundedMemory counts a single 32 MiB line produced on the fly. A
// solution that buffers whole lines (io.ReadAll, a Scanner with a huge
// buffer, ReadString) allocates at least 32 MiB; a streaming one allocates
// a few KiB.
func TestBoundedMemory(t *testing.T) {
	const size = 32 << 20
	r := io.LimitReader(repeatReader('q'), size)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	got, err := Count(r)
	runtime.ReadMemStats(&after)

	if err != nil {
		t.Fatalf("Count error = %v", err)
	}
	if want := (Stats{1, size}); got != want {
		t.Errorf("Count = %+v, want %+v", got, want)
	}
	const limit = 4 << 20
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > limit {
		t.Errorf("Count allocated %d bytes for a %d-byte line; want <= %d (stream, don't buffer whole lines)", alloc, size, limit)
	}
}

// repeatReader is an endless stream of one byte.
type repeatReader byte

func (r repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

func reference(s string) Stats {
	if s == "" {
		return Stats{}
	}
	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" { // input ended with "\n"
		lines = lines[:len(lines)-1]
	}
	var st Stats
	for i, l := range lines {
		terminated := i < len(lines)-1 || strings.HasSuffix(s, "\n")
		if terminated {
			l = strings.TrimSuffix(l, "\r")
		}
		st.Lines++
		st.Longest = max(st.Longest, len(l))
	}
	return st
}
