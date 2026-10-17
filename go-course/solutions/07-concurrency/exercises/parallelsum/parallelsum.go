// Package parallelsum splits a slice into chunks and sums them in parallel.
package parallelsum

import (
	"runtime"
	"sync"
)

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
	if n <= 0 {
		return nil
	}
	parts = max(1, min(parts, n))
	size, extra := n/parts, n%parts
	out := make([]Range, 0, parts)
	lo := 0
	for i := range parts {
		hi := lo + size
		if i < extra {
			hi++
		}
		out = append(out, Range{Lo: lo, Hi: hi})
		lo = hi
	}
	return out
}

// Sum returns the sum of nums, computed by splitting nums with Chunks and
// summing each chunk in its own goroutine. workers < 1 means
// runtime.GOMAXPROCS(0). Overflow wraps, exactly as a sequential loop would.
func Sum(nums []int64, workers int) int64 {
	if workers < 1 {
		workers = runtime.GOMAXPROCS(0)
	}
	chunks := Chunks(len(nums), workers)
	partials := make([]int64, len(chunks)) // one slot per goroutine: no shared writes

	var wg sync.WaitGroup
	for i, r := range chunks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var s int64 // accumulate locally; write the shared slice once
			for _, v := range nums[r.Lo:r.Hi] {
				s += v
			}
			partials[i] = s
		}()
	}
	wg.Wait() // happens-before: every partials[i] write is visible after Wait

	var total int64
	for _, p := range partials {
		total += p
	}
	return total
}
