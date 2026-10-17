package iterx

import (
	"slices"
	"testing"
)

func TestFilterMap(t *testing.T) {
	got := slices.Collect(FilterMap(slices.Values([]int{1, 2, 3, 4}), isEven, func(i int) int { return i * 10 }))
	if !slices.Equal(got, []int{20, 40}) {
		t.Errorf("FilterMap = %v, want [20 40]", got)
	}
	if got := firstN(t, FilterMap(Naturals(), isEven, func(i int) int { return i }), 2); !slices.Equal(got, []int{0, 2}) {
		t.Errorf("first 2 of FilterMap(Naturals()) = %v, want [0 2]", got)
	}
}

// BenchmarkSumEvenSquares sums the squares of the even numbers in 0..9999,
// three ways.
func BenchmarkSumEvenSquares(b *testing.B) {
	data := make([]int, 10_000)
	for i := range data {
		data[i] = i
	}
	square := func(i int) int { return i * i }
	want := 0
	for _, v := range data {
		if v%2 == 0 {
			want += v * v
		}
	}

	b.Run("pipeline", func(b *testing.B) {
		for b.Loop() {
			sum := 0
			for v := range Map(Filter(slices.Values(data), isEven), square) {
				sum += v
			}
			if sum != want {
				b.Fatal("wrong sum")
			}
		}
	})
	b.Run("fused", func(b *testing.B) {
		for b.Loop() {
			sum := 0
			for v := range FilterMap(slices.Values(data), isEven, square) {
				sum += v
			}
			if sum != want {
				b.Fatal("wrong sum")
			}
		}
	})
	b.Run("plain-loop", func(b *testing.B) {
		for b.Loop() {
			sum := 0
			for _, v := range data {
				if v%2 == 0 {
					sum += v * v
				}
			}
			if sum != want {
				b.Fatal("wrong sum")
			}
		}
	})
}
