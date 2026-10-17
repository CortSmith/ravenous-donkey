package shapes

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func randomShapes(n int) []Shape {
	r := rand.New(rand.NewPCG(1, 2))
	out := make([]Shape, n)
	for i := range out {
		switch i % 3 {
		case 0:
			out[i] = Circle{Radius: r.Float64() * 10}
		case 1:
			out[i] = Rect{Width: r.Float64() * 10, Height: r.Float64() * 10}
		default:
			a, b := 1+r.Float64()*5, 1+r.Float64()*5
			out[i] = Triangle{A: a, B: b, C: (a + b) / 2}
		}
	}
	return out
}

func TestFastMatchesReadable(t *testing.T) {
	for _, n := range []int{0, 1, 2, 10, 500} {
		in := randomShapes(n)
		// Add duplicate areas to exercise stability.
		in = append(in, Rect{Width: 2, Height: 3}, Rect{Width: 3, Height: 2}, Rect{Width: 6, Height: 1})

		a, b := slices.Clone(in), slices.Clone(in)
		SortByArea(a)
		SortByAreaFast(b)
		if !slices.Equal(a, b) {
			t.Fatalf("n=%d: SortByAreaFast differs from SortByArea", n)
		}

		g1, ok1 := Largest(in)
		g2, ok2 := LargestFast(in)
		if g1 != g2 || ok1 != ok2 {
			t.Fatalf("n=%d: LargestFast = %v, %v; Largest = %v, %v", n, g2, ok2, g1, ok1)
		}
	}
}

func BenchmarkSortByArea(b *testing.B) {
	src := randomShapes(1000)
	buf := make([]Shape, len(src))
	b.ReportAllocs()
	for b.Loop() {
		copy(buf, src)
		SortByArea(buf)
	}
}

func BenchmarkSortByAreaFast(b *testing.B) {
	src := randomShapes(1000)
	buf := make([]Shape, len(src))
	b.ReportAllocs()
	for b.Loop() {
		copy(buf, src)
		SortByAreaFast(buf)
	}
}

func BenchmarkLargest(b *testing.B) {
	src := randomShapes(1000)
	for b.Loop() {
		Largest(src)
	}
}

func BenchmarkLargestFast(b *testing.B) {
	src := randomShapes(1000)
	for b.Loop() {
		LargestFast(src)
	}
}
