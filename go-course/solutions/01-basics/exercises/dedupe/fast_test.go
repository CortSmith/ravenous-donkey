package dedupe

import (
	"fmt"
	"slices"
	"testing"
)

func TestDedupeFastMatchesDedupe(t *testing.T) {
	for n := range 100 {
		in := make([]string, n)
		for i := range in {
			in[i] = fmt.Sprint(i * 7 % (n/3 + 1))
		}
		if got, want := DedupeFast(in), Dedupe(in); !slices.Equal(got, want) {
			t.Fatalf("DedupeFast(%q) = %q, want %q", in, got, want)
		}
	}
}

func makeWords(n, distinct int) []string {
	s := make([]string, n)
	for i := range s {
		s[i] = fmt.Sprintf("word%d", i%distinct)
	}
	return s
}

func BenchmarkDedupe(b *testing.B) {
	for _, size := range []int{8, 32, 1000} {
		in := makeWords(size, size/2)
		b.Run(fmt.Sprintf("readable/n=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				Dedupe(in)
			}
		})
		b.Run(fmt.Sprintf("fast/n=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				DedupeFast(in)
			}
		})
	}
}
