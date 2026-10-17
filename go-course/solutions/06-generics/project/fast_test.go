package lru

import (
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

// TestFastMatchesReadable drives Cache and FastCache with the same random
// workload (including TTL expiry) and requires identical observable results.
func TestFastMatchesReadable(t *testing.T) {
	for _, ttl := range []time.Duration{0, 50 * time.Millisecond} {
		clk := newFakeClock()
		r := rand.New(rand.NewPCG(9, 9))
		slow := New[int, int](16, WithTTL(ttl), WithClock(clk.Now))
		fast := NewFast[int, int](16, WithTTL(ttl), WithClock(clk.Now))

		for op := range 50_000 {
			k := r.IntN(40)
			switch r.IntN(5) {
			case 0, 1:
				v := r.Int()
				slow.Put(k, v)
				fast.Put(k, v)
			case 2:
				sv, sok := slow.Get(k)
				fv, fok := fast.Get(k)
				if sv != fv || sok != fok {
					t.Fatalf("ttl=%v op %d: Get(%d): fast (%d, %v), readable (%d, %v)", ttl, op, k, fv, fok, sv, sok)
				}
			case 3:
				if s, f := slow.Delete(k), fast.Delete(k); s != f {
					t.Fatalf("ttl=%v op %d: Delete(%d): fast %v, readable %v", ttl, op, k, f, s)
				}
			case 4:
				clk.Advance(time.Duration(r.IntN(10)) * time.Millisecond)
			}
			if slow.Len() != fast.Len() {
				t.Fatalf("ttl=%v op %d: Len: fast %d, readable %d", ttl, op, fast.Len(), slow.Len())
			}
		}
		var sk, fk []int
		for k := range slow.All() {
			sk = append(sk, k)
		}
		for k := range fast.All() {
			fk = append(fk, k)
		}
		if !slices.Equal(sk, fk) {
			t.Errorf("ttl=%v: All keys: fast %v, readable %v", ttl, fk, sk)
		}
	}
}

func TestNewFastPanicsOnBadCapacity(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewFast(0) did not panic")
		}
	}()
	NewFast[int, int](0)
}

type lruCache interface {
	Get(int) (int, bool)
	Put(int, int)
}

// benchWorkload: capacity 1000, keys drawn from 0..1999 with a skew toward
// low keys, roughly 2 Gets per Put. Mix of hits, misses and evictions.
func benchWorkload(b *testing.B, c lruCache) {
	r := rand.New(rand.NewPCG(1, 1))
	keys := make([]int, 4096)
	for i := range keys {
		keys[i] = int(r.ExpFloat64()*600) % 2000
	}
	i := 0
	b.ReportAllocs()
	for b.Loop() {
		k := keys[i&4095]
		if _, ok := c.Get(k); !ok {
			c.Put(k, k)
		}
		i++
	}
}

func BenchmarkCache(b *testing.B) {
	b.Run("readable/list", func(b *testing.B) { benchWorkload(b, New[int, int](1000)) })
	b.Run("fast/arena", func(b *testing.B) { benchWorkload(b, NewFast[int, int](1000)) })
	b.Run("readable/list+ttl", func(b *testing.B) { benchWorkload(b, New[int, int](1000, WithTTL(time.Hour))) })
	b.Run("fast/arena+ttl", func(b *testing.B) { benchWorkload(b, NewFast[int, int](1000, WithTTL(time.Hour))) })
}
