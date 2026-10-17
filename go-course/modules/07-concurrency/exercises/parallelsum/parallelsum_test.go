package parallelsum

import (
	"math"
	"reflect"
	"testing"
	"time"
)

// callWithTimeout runs f on another goroutine and fails the test if it takes
// longer than d. f must only compute; assertions happen on the test goroutine.
func callWithTimeout[T any](t *testing.T, d time.Duration, f func() T) T {
	t.Helper()
	ch := make(chan T, 1) // buffered: a late result never blocks the goroutine forever
	go func() { ch <- f() }()
	select {
	case v := <-ch:
		return v
	case <-time.After(d):
		t.Fatalf("call did not return within %v (missing wg.Done? deadlock?)", d)
		panic("unreachable")
	}
}

func TestChunksExact(t *testing.T) {
	tests := []struct {
		n, parts int
		want     []Range
	}{
		{0, 4, nil},
		{-3, 4, nil},
		{1, 1, []Range{{0, 1}}},
		{10, 1, []Range{{0, 10}}},
		{10, 2, []Range{{0, 5}, {5, 10}}},
		{10, 3, []Range{{0, 4}, {4, 7}, {7, 10}}},
		{10, 4, []Range{{0, 3}, {3, 6}, {6, 8}, {8, 10}}},
		{3, 5, []Range{{0, 1}, {1, 2}, {2, 3}}},
		{5, 0, []Range{{0, 5}}},
		{5, -2, []Range{{0, 5}}},
	}
	for _, tt := range tests {
		got := Chunks(tt.n, tt.parts)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Chunks(%d, %d) = %v, want %v", tt.n, tt.parts, got, tt.want)
		}
	}
}

func TestChunksProperties(t *testing.T) {
	for n := 1; n <= 60; n++ {
		for parts := 1; parts <= 70; parts++ {
			got := Chunks(n, parts)
			wantLen := min(n, parts)
			if len(got) != wantLen {
				t.Fatalf("len(Chunks(%d, %d)) = %d, want %d", n, parts, len(got), wantLen)
			}
			next, minLen, maxLen := 0, n+1, -1
			for i, r := range got {
				if r.Lo != next {
					t.Fatalf("Chunks(%d, %d)[%d].Lo = %d, want %d (gap or overlap): %v", n, parts, i, r.Lo, next, got)
				}
				l := r.Hi - r.Lo
				if l <= 0 {
					t.Fatalf("Chunks(%d, %d)[%d] = %v is empty", n, parts, i, r)
				}
				if i > 0 && l > got[i-1].Hi-got[i-1].Lo {
					t.Fatalf("Chunks(%d, %d): range %d is longer than range %d; longer ranges must come first: %v", n, parts, i, i-1, got)
				}
				minLen, maxLen = min(minLen, l), max(maxLen, l)
				next = r.Hi
			}
			if next != n {
				t.Fatalf("Chunks(%d, %d) ends at %d, want %d: %v", n, parts, next, n, got)
			}
			if maxLen-minLen > 1 {
				t.Fatalf("Chunks(%d, %d) lengths differ by %d, want <= 1: %v", n, parts, maxLen-minLen, got)
			}
		}
	}
}

func seq(n int) []int64 {
	s := make([]int64, n)
	for i := range s {
		s[i] = int64(i + 1)
	}
	return s
}

func sequentialSum(nums []int64) int64 {
	var s int64
	for _, v := range nums {
		s += v
	}
	return s
}

func TestSum(t *testing.T) {
	tests := []struct {
		name    string
		nums    []int64
		workers int
		want    int64
	}{
		{"nil", nil, 4, 0},
		{"empty", []int64{}, 4, 0},
		{"one element", []int64{42}, 8, 42},
		{"negatives", []int64{-5, 3, -1, 10, -7}, 2, 0},
		{"1..100 one worker", seq(100), 1, 5050},
		{"1..100 seven workers", seq(100), 7, 5050},
		{"more workers than elements", seq(5), 64, 15},
		{"default workers (0)", seq(1000), 0, 500500},
		{"negative workers", seq(1000), -1, 500500},
		{"1..100000", seq(100_000), 16, 5_000_050_000},
		{"overflow wraps like sequential", []int64{math.MaxInt64, 1, 1}, 3, math.MinInt64 + 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := callWithTimeout(t, 5*time.Second, func() int64 { return Sum(tt.nums, tt.workers) })
			if got != tt.want {
				t.Errorf("Sum(len=%d, workers=%d) = %d, want %d", len(tt.nums), tt.workers, got, tt.want)
			}
		})
	}
}

func TestSumMatchesSequential(t *testing.T) {
	nums := make([]int64, 9973) // prime length so chunks are uneven
	for i := range nums {
		nums[i] = int64((i*7919)%2001 - 1000)
	}
	want := sequentialSum(nums)
	for workers := 1; workers <= 33; workers++ {
		got := callWithTimeout(t, 5*time.Second, func() int64 { return Sum(nums, workers) })
		if got != want {
			t.Errorf("Sum(nums, %d) = %d, want %d", workers, got, want)
		}
	}
}

func TestSumDoesNotModifyInput(t *testing.T) {
	nums := seq(50)
	orig := append([]int64(nil), nums...)
	callWithTimeout(t, 5*time.Second, func() int64 { return Sum(nums, 4) })
	if !reflect.DeepEqual(nums, orig) {
		t.Errorf("Sum modified its input: got %v, want %v", nums, orig)
	}
}
