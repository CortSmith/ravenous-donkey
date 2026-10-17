package funcs

import (
	"slices"
	"strconv"
	"testing"
)

func TestFastVariants(t *testing.T) {
	in := []int{1, 2, 3, 4, 5, 6}
	isEven := func(i int) bool { return i%2 == 0 }

	if got, want := MapFast(in, strconv.Itoa), Map(in, strconv.Itoa); !slices.Equal(got, want) {
		t.Errorf("MapFast = %q, want %q", got, want)
	}
	if got, want := FilterFast(in, isEven), Filter(in, isEven); !slices.Equal(got, want) {
		t.Errorf("FilterFast = %v, want %v", got, want)
	}
	cp := slices.Clone(in)
	if got := FilterInPlace(cp, isEven); !slices.Equal(got, []int{2, 4, 6}) {
		t.Errorf("FilterInPlace = %v, want [2 4 6]", got)
	}
}

func benchInput() []int {
	s := make([]int, 10_000)
	for i := range s {
		s[i] = i
	}
	return s
}

func BenchmarkMap(b *testing.B) {
	s := benchInput()
	f := func(i int) int { return i * 2 }
	b.Run("readable", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			Map(s, f)
		}
	})
	b.Run("fast", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			MapFast(s, f)
		}
	})
}

func BenchmarkFilter(b *testing.B) {
	s := benchInput()
	keep := func(i int) bool { return i%2 == 0 }
	b.Run("readable", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			Filter(s, keep)
		}
	})
	b.Run("prealloc", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			FilterFast(s, keep)
		}
	})
	b.Run("in-place", func(b *testing.B) {
		buf := make([]int, len(s))
		b.ReportAllocs()
		for b.Loop() {
			copy(buf, s) // in-place destroys its input, so refill each time
			FilterInPlace(buf, keep)
		}
	})
}
