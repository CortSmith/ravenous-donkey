package tail

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestTailSeekerMatchesTail(t *testing.T) {
	inputs := []string{
		"", "\n", "\n\n\n", "a", "a\n", "a\nb", "a\nb\n", "a\r\nb\r\n", "a\n\n\nb\n\n",
		strings.Repeat("0123456789\n", 20000),      // crosses many 64 KiB blocks
		strings.Repeat("x", 70000) + "\n" + "tail", // a line straddling a block boundary
	}
	for _, in := range inputs {
		for _, n := range []int{0, 1, 2, 3, 5, 100, 50000} {
			want, err := Tail(strings.NewReader(in), n)
			if err != nil {
				t.Fatal(err)
			}
			got, err := TailSeeker(strings.NewReader(in), n)
			if err != nil {
				t.Fatalf("TailSeeker(%.20q, %d) error = %v", in, n, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("TailSeeker(%.20q (%d newlines), %d) = %d lines, want %d lines", in, strings.Count(in, "\n"), n, len(got), len(want))
			}
			got, err = TailStream(strings.NewReader(in), n)
			if err != nil {
				t.Fatalf("TailStream(%.20q, %d) error = %v", in, n, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("TailStream(%.20q (%d newlines), %d) = %d lines, want %d lines", in, strings.Count(in, "\n"), n, len(got), len(want))
			}
		}
	}
}

func benchInput() string {
	var sb strings.Builder
	for i := range 200_000 {
		fmt.Fprintf(&sb, "2026-10-11T12:00:00Z INFO request served path=/api/v1/items/%d status=200\n", i)
	}
	return sb.String()
}

var input = benchInput()

func BenchmarkTail(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Tail(strings.NewReader(input), 10); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTailSeeker(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := TailSeeker(strings.NewReader(input), 10); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTailStream(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := TailStream(strings.NewReader(input), 10); err != nil {
			b.Fatal(err)
		}
	}
}
