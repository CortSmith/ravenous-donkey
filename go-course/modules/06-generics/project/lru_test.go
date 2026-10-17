package lru

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

// fakeClock is a manually advanced clock for TTL tests.
type fakeClock struct{ t time.Time }

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (f *fakeClock) Now() time.Time          { return f.t }
func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

// keys returns the keys of c in iteration order (most recent first).
func keys[K comparable, V any](c *Cache[K, V]) []K {
	var ks []K
	for k := range c.All() {
		ks = append(ks, k)
	}
	return ks
}

func assertGet[K comparable, V comparable](t *testing.T, c *Cache[K, V], key K, want V, wantOK bool) {
	t.Helper()
	got, ok := c.Get(key)
	if got != want || ok != wantOK {
		t.Errorf("Get(%v) = (%v, %v), want (%v, %v)", key, got, ok, want, wantOK)
	}
}

func assertKeys[K comparable, V any](t *testing.T, c *Cache[K, V], want ...K) {
	t.Helper()
	if got := keys(c); !slices.Equal(got, want) {
		t.Errorf("All() keys (most recent first) = %v, want %v", got, want)
	}
}

func TestNewPanicsOnBadCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("New(%d) did not panic, want panic for capacity < 1", capacity)
				}
			}()
			New[string, int](capacity)
		})
	}
}

func TestGetMissing(t *testing.T) {
	c := New[string, int](2)
	assertGet(t, c, "nope", 0, false)
	if c.Len() != 0 {
		t.Errorf("Len() = %d, want 0", c.Len())
	}
}

func TestPutGet(t *testing.T) {
	c := New[string, int](3)
	c.Put("a", 1)
	c.Put("b", 2)
	assertGet(t, c, "a", 1, true)
	assertGet(t, c, "b", 2, true)
	if c.Len() != 2 {
		t.Errorf("Len() = %d, want 2", c.Len())
	}
}

func TestEvictsLeastRecentlyUsed(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")    // a is now more recent than b
	c.Put("c", 3) // must evict b
	assertGet(t, c, "b", 0, false)
	assertGet(t, c, "a", 1, true)
	assertGet(t, c, "c", 3, true)
	if c.Len() != 2 {
		t.Errorf("Len() = %d after eviction, want 2 (capacity)", c.Len())
	}
}

func TestEvictionWithoutReads(t *testing.T) {
	c := New[int, string](3)
	for i := range 10 {
		c.Put(i, fmt.Sprint(i))
	}
	assertKeys(t, c, 9, 8, 7)
	if c.Len() != 3 {
		t.Errorf("Len() = %d, want 3", c.Len())
	}
}

func TestPutExistingUpdatesAndRefreshes(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("a", 10) // update: no growth, a becomes most recent
	if c.Len() != 2 {
		t.Fatalf("Len() = %d after updating an existing key, want 2", c.Len())
	}
	assertKeys(t, c, "a", "b")
	c.Put("c", 3) // evicts b, not a
	assertGet(t, c, "a", 10, true)
	assertGet(t, c, "b", 0, false)
}

func TestCapacityOne(t *testing.T) {
	c := New[string, string](1)
	c.Put("k1", "v1")
	c.Put("k2", "v2")
	assertGet(t, c, "k1", "", false)
	assertGet(t, c, "k2", "v2", true)
	c.Put("k2", "v2b")
	assertGet(t, c, "k2", "v2b", true)
	if c.Len() != 1 {
		t.Errorf("Len() = %d, want 1", c.Len())
	}
}

func TestDelete(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	if !c.Delete("a") {
		t.Error(`Delete("a") = false, want true`)
	}
	if c.Delete("a") {
		t.Error(`second Delete("a") = true, want false`)
	}
	if c.Delete("zzz") {
		t.Error(`Delete("zzz") = true, want false`)
	}
	if c.Len() != 1 {
		t.Errorf("Len() = %d after delete, want 1", c.Len())
	}
	// There is room again, so adding one entry must not evict b.
	c.Put("c", 3)
	assertGet(t, c, "b", 2, true)
	assertGet(t, c, "c", 3, true)
}

func TestAllOrderAndNoRecencyChange(t *testing.T) {
	c := New[string, int](3)
	assertKeys[string, int](t, c) // empty cache yields nothing

	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	c.Get("a")
	assertKeys(t, c, "a", "c", "b")

	// Values must travel with their keys.
	for k, v := range c.All() {
		if want := map[string]int{"a": 1, "b": 2, "c": 3}[k]; v != want {
			t.Errorf("All() yielded (%q, %d), want value %d", k, v, want)
		}
	}

	// Ranging over All must not touch recency: b is still the LRU entry.
	assertKeys(t, c, "a", "c", "b")
	c.Put("d", 4)
	assertGet(t, c, "b", 0, false)
}

func TestAllStopsEarlyAndIsReusable(t *testing.T) {
	c := New[int, int](5)
	for i := range 5 {
		c.Put(i, i*i)
	}
	calls := 0
	c.All()(func(int, int) bool {
		calls++
		return false
	})
	if calls != 1 {
		t.Errorf("All() called yield %d times after it returned false, want 1", calls)
	}

	seq := c.All()
	var first, second []int
	for k := range seq {
		first = append(first, k)
	}
	for k := range seq {
		second = append(second, k)
	}
	if !slices.Equal(first, []int{4, 3, 2, 1, 0}) || !slices.Equal(first, second) {
		t.Errorf("ranging over the same All() twice gave %v then %v, want [4 3 2 1 0] both times", first, second)
	}
}

func TestTTL(t *testing.T) {
	clk := newFakeClock()
	c := New[string, int](10, WithTTL(time.Minute), WithClock(clk.Now))

	c.Put("a", 1)
	clk.Advance(30 * time.Second)
	c.Put("b", 2)

	clk.Advance(29 * time.Second) // a is 59s old
	assertGet(t, c, "a", 1, true)

	clk.Advance(time.Second) // a is exactly 60s old: expired
	assertKeys(t, c, "b")    // All skips expired entries
	if c.Len() != 2 {
		t.Errorf("Len() = %d before expired entry is touched, want 2 (expired entries count until removed)", c.Len())
	}
	assertGet(t, c, "a", 0, false)
	if c.Len() != 1 {
		t.Errorf("Len() = %d after Get of an expired entry, want 1 (Get must remove it)", c.Len())
	}

	// Put on an existing key restarts its TTL.
	c.Put("b", 20) // b was 30s old; now fresh
	clk.Advance(45 * time.Second)
	assertGet(t, c, "b", 20, true)
	clk.Advance(15 * time.Second)
	assertGet(t, c, "b", 0, false)
}

func TestGetDoesNotRefreshTTL(t *testing.T) {
	clk := newFakeClock()
	c := New[string, int](10, WithTTL(time.Minute), WithClock(clk.Now))
	c.Put("a", 1)
	for range 5 {
		clk.Advance(10 * time.Second)
		c.Get("a")
	}
	clk.Advance(10 * time.Second) // 60s since Put
	assertGet(t, c, "a", 0, false)
}

func TestNoTTLNeverExpires(t *testing.T) {
	clk := newFakeClock()
	c := New[string, int](2, WithClock(clk.Now))
	c.Put("a", 1)
	clk.Advance(1000 * time.Hour)
	assertGet(t, c, "a", 1, true)

	// An explicit non-positive TTL also means "never".
	c2 := New[string, int](2, WithTTL(0), WithClock(clk.Now))
	c2.Put("a", 1)
	clk.Advance(1000 * time.Hour)
	assertGet(t, c2, "a", 1, true)
}

func TestStructValues(t *testing.T) {
	type user struct {
		Name string
		Age  int
	}
	c := New[int, user](2)
	c.Put(1, user{"ann", 30})
	assertGet(t, c, 1, user{"ann", 30}, true)
	assertGet(t, c, 2, user{}, false)
}

// TestAgainstModel runs a long random workload against the cache and a
// trivially correct (but O(n)) model and compares every observable result.
func TestAgainstModel(t *testing.T) {
	const capacity = 8
	r := rand.New(rand.NewPCG(42, 7)) // fixed seed: failures are reproducible
	c := New[int, int](capacity)
	var model []int // keys, most recent first
	vals := map[int]int{}

	touch := func(k int) {
		if i := slices.Index(model, k); i >= 0 {
			model = slices.Delete(model, i, i+1)
		}
		model = slices.Insert(model, 0, k)
	}

	for op := range 20_000 {
		k := r.IntN(capacity * 2)
		switch r.IntN(4) {
		case 0, 1: // put
			v := r.Int()
			c.Put(k, v)
			touch(k)
			vals[k] = v
			if len(model) > capacity {
				delete(vals, model[capacity])
				model = model[:capacity]
			}
		case 2: // get
			got, ok := c.Get(k)
			want, wantOK := vals[k]
			if got != want || ok != wantOK {
				t.Fatalf("op %d: Get(%d) = (%d, %v), want (%d, %v)", op, k, got, ok, want, wantOK)
			}
			if wantOK {
				touch(k)
			}
		case 3: // delete
			_, wantOK := vals[k]
			if got := c.Delete(k); got != wantOK {
				t.Fatalf("op %d: Delete(%d) = %v, want %v", op, k, got, wantOK)
			}
			if wantOK {
				model = slices.DeleteFunc(model, func(x int) bool { return x == k })
				delete(vals, k)
			}
		}
		if c.Len() != len(model) {
			t.Fatalf("op %d: Len() = %d, want %d", op, c.Len(), len(model))
		}
	}
	assertKeys(t, c, model...)
}
