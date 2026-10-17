package parallelsum

import (
	"runtime"
	"sync"
)

// minPerWorker is the smallest chunk worth a goroutine. Below this, starting
// and joining the goroutine costs more than the additions it saves.
// (Measured on the author's machine; re-measure on yours.)
const minPerWorker = 32 * 1024

// SumFast returns the same result as Sum, but
//   - caps the worker count so each goroutine gets at least minPerWorker
//     elements (small inputs run sequentially, with no goroutines at all), and
//   - uses four independent accumulators so the CPU can overlap additions
//     instead of waiting on one dependency chain.
func SumFast(nums []int64, workers int) int64 {
	if workers < 1 {
		workers = runtime.GOMAXPROCS(0)
	}
	workers = min(workers, len(nums)/minPerWorker)
	if workers <= 1 {
		return sum4(nums)
	}
	chunks := Chunks(len(nums), workers)
	partials := make([]int64, len(chunks))
	var wg sync.WaitGroup
	wg.Add(len(chunks) - 1)
	for i, r := range chunks[1:] {
		go func() {
			defer wg.Done()
			partials[i+1] = sum4(nums[r.Lo:r.Hi])
		}()
	}
	partials[0] = sum4(nums[chunks[0].Lo:chunks[0].Hi]) // the caller does a share too
	wg.Wait()
	var total int64
	for _, p := range partials {
		total += p
	}
	return total
}

func sum4(s []int64) int64 {
	var a, b, c, d int64
	for len(s) >= 4 {
		a += s[0]
		b += s[1]
		c += s[2]
		d += s[3]
		s = s[4:]
	}
	for _, v := range s {
		a += v
	}
	return a + b + c + d
}

// sumFalseSharing is the tempting-but-slow variant: every goroutine adds
// directly into partials[i]. Neighbouring slots share a cache line, so each
// write invalidates the other cores' copies. It is correct (no data race,
// each goroutine owns its own index) but scales badly. Benchmarks only.
func sumFalseSharing(nums []int64, workers int) int64 {
	chunks := Chunks(len(nums), workers)
	partials := make([]int64, len(chunks))
	var wg sync.WaitGroup
	for i, r := range chunks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, v := range nums[r.Lo:r.Hi] {
				partials[i] += v
			}
		}()
	}
	wg.Wait()
	var total int64
	for _, p := range partials {
		total += p
	}
	return total
}
