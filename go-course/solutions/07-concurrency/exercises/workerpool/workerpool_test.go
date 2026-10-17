package workerpool

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mapResult[T any] struct {
	out []T
	err error
}

// mapWithTimeout calls Map on another goroutine and fails the test if it
// does not return within d, so a deadlocked implementation fails fast
// instead of hanging the whole test binary.
func mapWithTimeout[In, Out any](t *testing.T, d time.Duration, ctx context.Context, in []In, workers int, fn func(context.Context, In) (Out, error)) ([]Out, error) {
	t.Helper()
	ch := make(chan mapResult[Out], 1)
	go func() {
		out, err := Map(ctx, in, workers, fn)
		ch <- mapResult[Out]{out, err}
	}()
	select {
	case r := <-ch:
		return r.out, r.err
	case <-time.After(d):
		t.Fatalf("Map did not return within %v (deadlock, or not waiting on ctx.Done()?)", d)
		return nil, nil
	}
}

// checkNoLeak fails if the goroutine count does not return to base.
func checkNoLeak(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			n := runtime.Stack(buf, true)
			t.Errorf("goroutine leak: %d goroutines running, want <= %d\n%s", runtime.NumGoroutine(), base, buf[:n])
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func ints(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

func TestMapPreservesOrder(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		workers int
	}{
		{"one worker", 20, 1},
		{"four workers", 50, 4},
		{"workers equal jobs", 8, 8},
		{"more workers than jobs", 3, 16},
		{"single job", 1, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := runtime.NumGoroutine()
			in := ints(tt.n)
			got, err := mapWithTimeout(t, 5*time.Second, t.Context(), in, tt.workers,
				func(_ context.Context, v int) (string, error) {
					// Later jobs finish first, so a pool that appends results
					// in completion order will produce the wrong order.
					time.Sleep(time.Duration(tt.n-v) * 100 * time.Microsecond)
					return "item-" + strconv.Itoa(v), nil
				})
			if err != nil {
				t.Fatalf("Map(%d jobs, %d workers) error = %v, want nil", tt.n, tt.workers, err)
			}
			if len(got) != tt.n {
				t.Fatalf("Map(%d jobs, %d workers) returned %d results, want %d", tt.n, tt.workers, len(got), tt.n)
			}
			for i, s := range got {
				if want := "item-" + strconv.Itoa(i); s != want {
					t.Errorf("result[%d] = %q, want %q (results must be in input order)", i, s, want)
				}
			}
			checkNoLeak(t, base)
		})
	}
}

func TestMapEmptyInput(t *testing.T) {
	calls := 0
	got, err := mapWithTimeout(t, 2*time.Second, t.Context(), []int{}, 4,
		func(context.Context, int) (int, error) { calls++; return 0, nil })
	if err != nil {
		t.Errorf("Map(empty) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("Map(empty) = %v, want empty", got)
	}
	if calls != 0 {
		t.Errorf("Map(empty) called fn %d times, want 0", calls)
	}
}

func TestMapInvalidWorkers(t *testing.T) {
	for _, w := range []int{0, -1, -100} {
		got, err := mapWithTimeout(t, 2*time.Second, t.Context(), ints(3), w,
			func(_ context.Context, v int) (int, error) { return v, nil })
		if !errors.Is(err, ErrInvalidWorkers) {
			t.Errorf("Map(workers=%d) error = %v, want ErrInvalidWorkers", w, err)
		}
		if got != nil {
			t.Errorf("Map(workers=%d) = %v, want nil results with an error", w, got)
		}
	}
}

func TestMapCallsEachElementOnce(t *testing.T) {
	const n = 200
	var mu sync.Mutex
	seen := make(map[int]int)
	_, err := mapWithTimeout(t, 5*time.Second, t.Context(), ints(n), 7,
		func(_ context.Context, v int) (int, error) {
			mu.Lock()
			seen[v]++
			mu.Unlock()
			return v * v, nil
		})
	if err != nil {
		t.Fatalf("Map error = %v, want nil", err)
	}
	for i := range n {
		if seen[i] != 1 {
			t.Errorf("fn called %d times for element %d, want exactly 1", seen[i], i)
		}
	}
}

func TestMapUsesExactlyWorkersGoroutines(t *testing.T) {
	for _, workers := range []int{1, 3, 6} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			var active, peak atomic.Int32
			allBusy := make(chan struct{})
			var once sync.Once
			fn := func(ctx context.Context, v int) (int, error) {
				n := active.Add(1)
				defer active.Add(-1)
				for {
					p := peak.Load()
					if n <= p || peak.CompareAndSwap(p, n) {
						break
					}
				}
				if int(n) == workers {
					once.Do(func() { close(allBusy) })
				}
				// Wait until every worker is busy at once (or give up), so the
				// test sees the true concurrency level without relying on luck.
				select {
				case <-allBusy:
				case <-time.After(500 * time.Millisecond):
				}
				return v, nil
			}
			_, err := mapWithTimeout(t, 10*time.Second, t.Context(), ints(workers*4), workers, fn)
			if err != nil {
				t.Fatalf("Map error = %v, want nil", err)
			}
			if got := int(peak.Load()); got > workers {
				t.Errorf("peak concurrent calls = %d, want <= %d (pool is not bounded)", got, workers)
			} else if got < workers {
				t.Errorf("peak concurrent calls = %d, want %d (pool is not running workers in parallel)", got, workers)
			}
		})
	}
}

var errBoom = errors.New("boom")

func TestMapStopsOnFirstError(t *testing.T) {
	base := runtime.NumGoroutine()
	var calls atomic.Int32
	got, err := mapWithTimeout(t, 5*time.Second, t.Context(), ints(100), 1,
		func(_ context.Context, v int) (int, error) {
			calls.Add(1)
			if v == 2 {
				return 0, fmt.Errorf("job %d: %w", v, errBoom)
			}
			return v, nil
		})
	if !errors.Is(err, errBoom) {
		t.Fatalf("Map error = %v, want an error wrapping errBoom", err)
	}
	if got != nil {
		t.Errorf("Map results = %v, want nil when an error occurs", got)
	}
	if c := calls.Load(); c != 3 {
		t.Errorf("fn called %d times with 1 worker failing on job 2, want 3 (no new jobs after an error)", c)
	}
	checkNoLeak(t, base)
}

func TestMapErrorCancelsInFlightCalls(t *testing.T) {
	base := runtime.NumGoroutine()
	var sawCancel atomic.Int32
	_, err := mapWithTimeout(t, 5*time.Second, t.Context(), ints(8), 4,
		func(ctx context.Context, v int) (int, error) {
			if v == 0 {
				time.Sleep(20 * time.Millisecond) // let the others start blocking
				return 0, errBoom
			}
			select {
			case <-ctx.Done():
				sawCancel.Add(1)
				return 0, ctx.Err()
			case <-time.After(3 * time.Second):
				return v, nil
			}
		})
	if !errors.Is(err, errBoom) {
		t.Fatalf("Map error = %v, want errBoom (the FIRST error, not a later context.Canceled)", err)
	}
	if sawCancel.Load() == 0 {
		t.Error("no in-flight call observed ctx.Done(); the ctx passed to fn must be canceled after the first error")
	}
	checkNoLeak(t, base)
}

func TestMapParentContextCanceled(t *testing.T) {
	t.Run("before start", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := mapWithTimeout(t, 5*time.Second, ctx, ints(50), 4,
			func(ctx context.Context, v int) (int, error) { return v, nil })
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Map with canceled ctx error = %v, want context.Canceled", err)
		}
		checkNoLeak(t, base)
	})
	t.Run("during run", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var calls atomic.Int32
		_, err := mapWithTimeout(t, 5*time.Second, ctx, ints(1000), 2,
			func(ctx context.Context, v int) (int, error) {
				if calls.Add(1) == 5 {
					cancel()
				}
				select {
				case <-ctx.Done():
					return 0, ctx.Err()
				case <-time.After(time.Millisecond):
					return v, nil
				}
			})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Map error = %v, want context.Canceled", err)
		}
		if c := calls.Load(); c > 50 {
			t.Errorf("fn called %d times after cancellation at call 5, want <= 50 (stop feeding jobs when ctx is done)", c)
		}
		checkNoLeak(t, base)
	})
	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
		defer cancel()
		_, err := mapWithTimeout(t, 5*time.Second, ctx, ints(1000), 2,
			func(ctx context.Context, v int) (int, error) {
				select {
				case <-ctx.Done():
					return 0, ctx.Err()
				case <-time.After(5 * time.Millisecond):
					return v, nil
				}
			})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Map error = %v, want context.DeadlineExceeded", err)
		}
	})
}
