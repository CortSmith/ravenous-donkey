// Package parallelsum splits a slice into chunks and sums them in parallel.
package parallelsum

// Range is the half-open index interval [Lo, Hi).
type Range struct {
	Lo, Hi int
}

// Chunks splits the indices [0, n) into contiguous, non-overlapping ranges
// that cover every index exactly once. It returns min(parts, n) ranges whose
// lengths differ by at most one; when they cannot all be equal, the earlier
// ranges are the longer ones. parts < 1 is treated as 1. Chunks returns nil
// when n <= 0.
func Chunks(n, parts int) []Range {
	// TODO: implement
	return nil
}

// Sum returns the sum of nums, computed by splitting nums with Chunks and
// summing each chunk in its own goroutine. workers < 1 means
// runtime.GOMAXPROCS(0). Overflow wraps, exactly as a sequential loop would.
func Sum(nums []int64, workers int) int64 {
	// TODO: implement
	return 0
}
