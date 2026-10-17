package counter

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Run these tests with the race detector:
//
//	go test -race ./modules/07-concurrency/exercises/counter/

// incrementer is the method set shared by Counter and AtomicCounter.
type incrementer interface {
	Inc()
	Add(delta int64)
	Value() int64
}

// Compile-time checks that both types implement the interface with pointer receivers.
var (
	_ incrementer = (*Counter)(nil)
	_ incrementer = (*AtomicCounter)(nil)
)

// runWithTimeout runs f in a goroutine and fails the test if it does not
// finish within d. f must not call t.Fatal (it runs on another goroutine);
// it only does work, and the caller checks results afterwards.
func runWithTimeout(t *testing.T, d time.Duration, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("operation did not finish within %v (deadlock or lock never released?)", d)
	}
}

func counters() map[string]func() incrementer {
	return map[string]func() incrementer{
		"Counter":       func() incrementer { return &Counter{} },
		"AtomicCounter": func() incrementer { return &AtomicCounter{} },
	}
}

func TestZeroValue(t *testing.T) {
	for name, newC := range counters() {
		t.Run(name, func(t *testing.T) {
			c := newC()
			if got := c.Value(); got != 0 {
				t.Errorf("zero %s.Value() = %d, want 0", name, got)
			}
		})
	}
}

func TestSequential(t *testing.T) {
	tests := []struct {
		name string
		ops  []int64 // 0 means Inc(), anything else means Add(op)
		want int64
	}{
		{"single inc", []int64{0}, 1},
		{"three incs", []int64{0, 0, 0}, 3},
		{"add positive", []int64{5}, 5},
		{"add negative", []int64{-7}, -7},
		{"mixed", []int64{0, 10, -3, 0, -9}, 0},
		{"large", []int64{1 << 40, 1 << 40}, 1 << 41},
	}
	for name, newC := range counters() {
		for _, tt := range tests {
			t.Run(name+"/"+tt.name, func(t *testing.T) {
				c := newC()
				for _, op := range tt.ops {
					if op == 0 {
						c.Inc()
					} else {
						c.Add(op)
					}
				}
				if got := c.Value(); got != tt.want {
					t.Errorf("after ops %v: Value() = %d, want %d", tt.ops, got, tt.want)
				}
			})
		}
	}
}

func TestConcurrentIncrements(t *testing.T) {
	const (
		goroutines = 50
		perG       = 1000
	)
	for name, newC := range counters() {
		t.Run(name, func(t *testing.T) {
			c := newC()
			runWithTimeout(t, 10*time.Second, func() {
				var wg sync.WaitGroup
				for g := range goroutines {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for range perG {
							if g%2 == 0 {
								c.Inc()
							} else {
								c.Add(1)
							}
							_ = c.Value() // concurrent reads must be safe too
						}
					}()
				}
				wg.Wait()
			})
			if got, want := c.Value(), int64(goroutines*perG); got != want {
				t.Errorf("%s.Value() after %d concurrent increments = %d, want %d (lost updates mean missing synchronization)",
					name, goroutines*perG, got, want)
			}
		})
	}
}

func TestConcurrentAddAndSubtract(t *testing.T) {
	for name, newC := range counters() {
		t.Run(name, func(t *testing.T) {
			c := newC()
			c.Add(100)
			runWithTimeout(t, 10*time.Second, func() {
				var wg sync.WaitGroup
				for i := range 40 {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for range 500 {
							if i%2 == 0 {
								c.Add(3)
							} else {
								c.Add(-3)
							}
						}
					}()
				}
				wg.Wait()
			})
			if got := c.Value(); got != 100 {
				t.Errorf("%s.Value() = %d, want 100 (equal numbers of +3 and -3)", name, got)
			}
		})
	}
}

func TestKeyedCounterZeroValue(t *testing.T) {
	var c KeyedCounter
	if got := c.Get("missing"); got != 0 {
		t.Errorf("zero KeyedCounter.Get(%q) = %d, want 0", "missing", got)
	}
	snap := c.Snapshot()
	if snap == nil {
		t.Fatal("zero KeyedCounter.Snapshot() = nil, want empty non-nil map")
	}
	if len(snap) != 0 {
		t.Errorf("zero KeyedCounter.Snapshot() = %v, want empty", snap)
	}
	c.Inc("a") // must not panic on a nil map
	if got := c.Get("a"); got != 1 {
		t.Errorf("Get(%q) after one Inc = %d, want 1", "a", got)
	}
}

func TestKeyedCounterSequential(t *testing.T) {
	var c KeyedCounter
	for _, k := range []string{"go", "rust", "go", "", "go", ""} {
		c.Inc(k)
	}
	want := map[string]int64{"go": 3, "rust": 1, "": 2}
	for k, w := range want {
		if got := c.Get(k); got != w {
			t.Errorf("Get(%q) = %d, want %d", k, got, w)
		}
	}
	got := c.Snapshot()
	if len(got) != len(want) {
		t.Fatalf("Snapshot() = %v, want %v", got, want)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("Snapshot()[%q] = %d, want %d", k, got[k], w)
		}
	}
}

func TestKeyedCounterSnapshotIsCopy(t *testing.T) {
	var c KeyedCounter
	c.Inc("x")
	snap := c.Snapshot()
	snap["x"] = 100
	snap["y"] = 5
	if got := c.Get("x"); got != 1 {
		t.Errorf("after mutating snapshot, Get(%q) = %d, want 1 (Snapshot must return a copy)", "x", got)
	}
	if got := c.Get("y"); got != 0 {
		t.Errorf("after mutating snapshot, Get(%q) = %d, want 0 (Snapshot must return a copy)", "y", got)
	}
	c.Inc("x")
	if snap["x"] != 100 {
		t.Errorf("later Inc changed an earlier snapshot: snap[%q] = %d, want 100", "x", snap["x"])
	}
}

func TestKeyedCounterConcurrent(t *testing.T) {
	const (
		goroutines = 20
		perG       = 500
		keys       = 7
	)
	var c KeyedCounter
	runWithTimeout(t, 10*time.Second, func() {
		var wg sync.WaitGroup
		for g := range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range perG {
					c.Inc(fmt.Sprintf("k%d", (g+i)%keys))
					if i%50 == 0 {
						// Iterating the snapshot while others write must be race-free.
						for range c.Snapshot() {
						}
						_ = c.Get("k0")
					}
				}
			}()
		}
		wg.Wait()
	})
	var total int64
	for k, v := range c.Snapshot() {
		if v <= 0 {
			t.Errorf("Snapshot()[%q] = %d, want > 0", k, v)
		}
		total += v
	}
	if want := int64(goroutines * perG); total != want {
		t.Errorf("sum of all counts = %d, want %d (lost updates)", total, want)
	}
	if got := len(c.Snapshot()); got != keys {
		t.Errorf("len(Snapshot()) = %d, want %d", got, keys)
	}
}
