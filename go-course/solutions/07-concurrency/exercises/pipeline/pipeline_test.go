package pipeline

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"testing"
	"time"
)

const timeout = 3 * time.Second

// collect reads ch until it is closed. It fails the test if ch is not closed
// within d, instead of blocking forever on a stage that never closes.
func collect(t *testing.T, ch <-chan int, d time.Duration) []int {
	t.Helper()
	if ch == nil {
		t.Fatal("stage returned a nil channel; receiving from it would block forever")
	}
	var got []int
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, v)
		case <-timer.C:
			t.Fatalf("channel not closed within %v (received %v so far); does the stage close its output?", d, got)
		}
	}
}

// recv receives one value, failing the test on timeout or a closed channel.
func recv(t *testing.T, ch <-chan int) int {
	t.Helper()
	select {
	case v, ok := <-ch:
		if !ok {
			t.Fatal("channel closed early, want a value")
		}
		return v
	case <-time.After(timeout):
		t.Fatalf("no value received within %v", timeout)
	}
	return 0
}

// checkNoLeak waits for the goroutine count to drop back to base.
func checkNoLeak(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			n := runtime.Stack(buf, true)
			t.Errorf("goroutine leak: %d goroutines, want <= %d\n%s", runtime.NumGoroutine(), base, buf[:n])
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// source returns a channel fed with nums by a goroutine the test owns.
func source(nums ...int) <-chan int {
	ch := make(chan int, len(nums))
	for _, n := range nums {
		ch <- n
	}
	close(ch)
	return ch
}

func TestGenerate(t *testing.T) {
	tests := [][]int{nil, {7}, {1, 2, 3}, {5, -1, 0, 5}}
	for _, nums := range tests {
		got := collect(t, Generate(t.Context(), nums...), timeout)
		if !slices.Equal(got, nums) {
			t.Errorf("Generate(%v) yielded %v, want %v", nums, got, nums)
		}
	}
}

func TestGenerateStopsOnCancel(t *testing.T) {
	base := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(t.Context())
	nums := make([]int, 10_000)
	ch := Generate(ctx, nums...)
	recv(t, ch)
	cancel()
	// Nobody reads any more: the generator must notice ctx.Done() while
	// blocked on its send and exit on its own.
	checkNoLeak(t, base)
	rest := collect(t, ch, timeout)
	if len(rest) > 1 {
		t.Errorf("after cancel, Generate yielded %d more values, want at most 1", len(rest))
	}
}

func TestSquare(t *testing.T) {
	tests := []struct {
		in, want []int
	}{
		{nil, nil},
		{[]int{3}, []int{9}},
		{[]int{1, 2, 3, 4}, []int{1, 4, 9, 16}},
		{[]int{-3, 0, 3}, []int{9, 0, 9}},
	}
	for _, tt := range tests {
		got := collect(t, Square(t.Context(), source(tt.in...)), timeout)
		if !slices.Equal(got, tt.want) {
			t.Errorf("Square(%v) yielded %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestSquareStopsWhenBlockedOnReceive(t *testing.T) {
	base := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(t.Context())
	in := make(chan int) // never sent on, never closed until cleanup
	t.Cleanup(func() { close(in) })
	out := Square(ctx, in)
	cancel()
	// Square is blocked receiving from in; it must also watch ctx.Done().
	if got := collect(t, out, timeout); len(got) != 0 {
		t.Errorf("Square yielded %v from an empty input, want nothing", got)
	}
	checkNoLeak(t, base)
}

func TestSquareStopsWhenBlockedOnSend(t *testing.T) {
	base := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(t.Context())
	out := Square(ctx, source(1, 2, 3, 4, 5))
	if v := recv(t, out); v != 1 {
		t.Errorf("first value = %d, want 1", v)
	}
	cancel()
	// Nobody reads out any more; Square must exit while blocked on its send.
	checkNoLeak(t, base)
}

func TestMerge(t *testing.T) {
	got := collect(t, Merge(t.Context(), source(1, 2, 3), source(), source(10, 20), source(100)), timeout)
	slices.Sort(got)
	want := []int{1, 2, 3, 10, 20, 100}
	if !slices.Equal(got, want) {
		t.Errorf("Merge yielded (sorted) %v, want %v", got, want)
	}
}

func TestMergePreservesPerChannelOrder(t *testing.T) {
	a := []int{1, 3, 5, 7, 9, 11, 13}
	b := []int{2, 4, 6, 8, 10, 12, 14}
	got := collect(t, Merge(t.Context(), source(a...), source(b...)), timeout)
	var gotA, gotB []int
	for _, v := range got {
		if v%2 == 1 {
			gotA = append(gotA, v)
		} else {
			gotB = append(gotB, v)
		}
	}
	if !slices.Equal(gotA, a) || !slices.Equal(gotB, b) {
		t.Errorf("Merge reordered values within one input: odd %v, even %v", gotA, gotB)
	}
}

func TestMergeNoChannels(t *testing.T) {
	if got := collect(t, Merge(t.Context()), timeout); len(got) != 0 {
		t.Errorf("Merge() yielded %v, want nothing", got)
	}
}

func TestMergeStopsOnCancel(t *testing.T) {
	base := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(t.Context())
	blocked := make(chan int) // never sends
	t.Cleanup(func() { close(blocked) })
	out := Merge(ctx, source(1, 2, 3, 4), blocked)
	recv(t, out)
	cancel()
	// One forwarder is blocked sending to out, another is blocked receiving
	// from blocked. Both must exit, and out must then be closed.
	checkNoLeak(t, base)
	collect(t, out, timeout)
}

func TestSumSquares(t *testing.T) {
	nums := make([]int, 1000)
	for i := range nums {
		nums[i] = i + 1
	}
	const want = 1000 * 1001 * 2001 / 6 // sum of squares 1..n
	tests := []struct {
		name    string
		nums    []int
		workers int
		want    int
	}{
		{"empty", nil, 3, 0},
		{"one", []int{-4}, 1, 16},
		{"1..1000 one worker", nums, 1, want},
		{"1..1000 three workers", nums, 3, want},
		{"1..1000 sixteen workers", nums, 16, want},
		{"zero workers means one", nums, 0, want},
		{"negative workers means one", []int{1, 2}, -5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := runtime.NumGoroutine()
			n, err := callSumSquares(t, t.Context(), tt.nums, tt.workers)
			if err != nil || n != tt.want {
				t.Errorf("SumSquares(len=%d, %d) = %d, %v; want %d, nil", len(tt.nums), tt.workers, n, err, tt.want)
			}
			checkNoLeak(t, base)
		})
	}
}

func TestSumSquaresCanceled(t *testing.T) {
	t.Run("before start", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		n, err := callSumSquares(t, ctx, make([]int, 100), 4)
		if !errors.Is(err, context.Canceled) || n != 0 {
			t.Errorf("SumSquares(canceled ctx) = %d, %v; want 0, context.Canceled", n, err)
		}
		checkNoLeak(t, base)
	})
	t.Run("midway", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Millisecond)
		defer cancel()
		nums := make([]int, 5_000_000) // far too many to finish in 5ms
		n, err := callSumSquares(t, ctx, nums, 4)
		if !errors.Is(err, context.DeadlineExceeded) || n != 0 {
			t.Errorf("SumSquares(5ms deadline, 5M numbers) = %d, %v; want 0, context.DeadlineExceeded", n, err)
		}
		checkNoLeak(t, base)
	})
}

func callSumSquares(t *testing.T, ctx context.Context, nums []int, workers int) (int, error) {
	t.Helper()
	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		n, err := SumSquares(ctx, nums, workers)
		ch <- result{n, err}
	}()
	select {
	case r := <-ch:
		return r.n, r.err
	case <-time.After(timeout):
		t.Fatalf("SumSquares did not return within %v of its context being done", timeout)
	}
	return 0, nil
}
