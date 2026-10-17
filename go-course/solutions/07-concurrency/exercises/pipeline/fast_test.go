package pipeline

import (
	"context"
	"errors"
	"testing"
)

func TestSumSquaresBatched(t *testing.T) {
	for _, n := range []int{0, 1, batchSize - 1, batchSize, batchSize + 1, 100_000} {
		nums := make([]int, n)
		for i := range nums {
			nums[i] = i%2001 - 1000
		}
		want, err := SumSquares(t.Context(), nums, 4)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range []int{0, 1, 4, 32} {
			got, err := SumSquaresBatched(t.Context(), nums, w)
			if err != nil || got != want {
				t.Errorf("SumSquaresBatched(len=%d, %d) = %d, %v; want %d, nil", n, w, got, err, want)
			}
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := SumSquaresBatched(ctx, make([]int, 10_000), 4); !errors.Is(err, context.Canceled) {
		t.Errorf("SumSquaresBatched(canceled) error = %v, want context.Canceled", err)
	}
}

func BenchmarkSumSquares(b *testing.B) {
	nums := make([]int, 100_000)
	for i := range nums {
		nums[i] = i
	}
	b.Run("channels-per-item", func(b *testing.B) {
		for b.Loop() {
			SumSquares(context.Background(), nums, 4)
		}
	})
	b.Run("batched", func(b *testing.B) {
		for b.Loop() {
			SumSquaresBatched(context.Background(), nums, 4)
		}
	})
}
