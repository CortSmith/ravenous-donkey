package parallelsum

import (
	"fmt"
	"testing"
)

func TestSumFast(t *testing.T) {
	for _, n := range []int{0, 1, 3, 4, 5, 1000, minPerWorker*3 + 7, 1_000_003} {
		nums := seq(n)
		want := sequentialSum(nums)
		for _, w := range []int{0, 1, 2, 8, 64} {
			if got := SumFast(nums, w); got != want {
				t.Errorf("SumFast(len=%d, %d) = %d, want %d", n, w, got, want)
			}
			if got := sumFalseSharing(nums, max(w, 1)); got != want {
				t.Errorf("sumFalseSharing(len=%d, %d) = %d, want %d", n, w, got, want)
			}
		}
	}
}

func BenchmarkSum(b *testing.B) {
	for _, n := range []int{1_000, 100_000, 10_000_000} {
		nums := seq(n)
		b.Run(fmt.Sprintf("n=%d/sequential", n), func(b *testing.B) {
			for b.Loop() {
				sequentialSum(nums)
			}
		})
		b.Run(fmt.Sprintf("n=%d/Sum", n), func(b *testing.B) {
			for b.Loop() {
				Sum(nums, 0)
			}
		})
		b.Run(fmt.Sprintf("n=%d/SumFast", n), func(b *testing.B) {
			for b.Loop() {
				SumFast(nums, 0)
			}
		})
		b.Run(fmt.Sprintf("n=%d/falseSharing", n), func(b *testing.B) {
			for b.Loop() {
				sumFalseSharing(nums, 8)
			}
		})
	}
}
