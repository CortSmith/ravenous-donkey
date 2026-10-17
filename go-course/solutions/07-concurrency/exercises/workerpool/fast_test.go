package workerpool

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestMapFastMatchesMap(t *testing.T) {
	sq := func(_ context.Context, v int) (int, error) { return v * v, nil }
	for _, n := range []int{0, 1, 7, 1000} {
		for _, w := range []int{1, 3, 16} {
			want, err1 := Map(t.Context(), ints(n), w, sq)
			got, err2 := MapFast(t.Context(), ints(n), w, sq)
			if err1 != nil || err2 != nil || !slices.Equal(got, want) {
				t.Errorf("n=%d w=%d: MapFast = %v, %v; Map = %v, %v", n, w, got, err2, want, err1)
			}
		}
	}
	_, err := MapFast(t.Context(), ints(100), 4, func(_ context.Context, v int) (int, error) {
		if v == 50 {
			return 0, errBoom
		}
		return v, nil
	})
	if !errors.Is(err, errBoom) {
		t.Errorf("MapFast error = %v, want errBoom", err)
	}
	if _, err := MapFast(t.Context(), ints(3), 0, sq); !errors.Is(err, ErrInvalidWorkers) {
		t.Errorf("MapFast(workers=0) error = %v, want ErrInvalidWorkers", err)
	}
}

func BenchmarkMap(b *testing.B) {
	in := ints(10_000)
	cheap := func(_ context.Context, v int) (int, error) { return v * 2, nil }
	b.Run("Map", func(b *testing.B) {
		for b.Loop() {
			Map(context.Background(), in, 8, cheap)
		}
	})
	b.Run("MapFast", func(b *testing.B) {
		for b.Loop() {
			MapFast(context.Background(), in, 8, cheap)
		}
	})
}
