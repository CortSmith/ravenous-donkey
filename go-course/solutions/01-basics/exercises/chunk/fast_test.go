package chunk

import (
	"slices"
	"testing"
)

func TestRotateLeftFastMatches(t *testing.T) {
	for n := range 12 {
		for k := -2 * n; k <= 2*n+1; k++ {
			a := make([]int, n)
			for i := range a {
				a[i] = i
			}
			b := slices.Clone(a)
			RotateLeft(a, k)
			RotateLeftFast(b, k)
			if !slices.Equal(a, b) {
				t.Fatalf("n=%d k=%d: RotateLeftFast gave %v, RotateLeft gave %v", n, k, b, a)
			}
		}
	}
}

func TestRotateLeftFastDoesNotAllocate(t *testing.T) {
	s := make([]int, 1000)
	if allocs := testing.AllocsPerRun(100, func() { RotateLeftFast(s, 333) }); allocs != 0 {
		t.Errorf("RotateLeftFast allocated %.0f times, want 0", allocs)
	}
}

var benchSlice = make([]int, 10_000)

func BenchmarkRotateLeft(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		RotateLeft(benchSlice, 3_333)
	}
}

func BenchmarkRotateLeftFast(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		RotateLeftFast(benchSlice, 3_333)
	}
}
