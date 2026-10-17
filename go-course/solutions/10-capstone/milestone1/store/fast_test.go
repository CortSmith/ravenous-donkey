package store

import (
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"
)

func TestShardedStoreMatchesStore(t *testing.T) {
	clk := newFakeClock()
	s := NewSharded(clk.Now)
	for i := range 100 {
		ttl := time.Duration(0)
		if i%2 == 0 {
			ttl = time.Second
		}
		s.Set(strconv.Itoa(i), "v"+strconv.Itoa(i), ttl)
	}
	if v, err := s.Get("7"); err != nil || v != "v7" {
		t.Fatalf("Get(\"7\") = (%q, %v), want (\"v7\", nil)", v, err)
	}
	clk.Advance(time.Second)
	if _, err := s.Get("8"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(\"8\") after expiry error = %v, want ErrNotFound", err)
	}
	if got := len(s.Keys()); got != 50 {
		t.Errorf("len(Keys()) = %d, want 50", got)
	}
	if n := s.DeleteExpired(); n != 50 {
		t.Errorf("DeleteExpired() = %d, want 50", n)
	}
	if !s.Delete("1") || s.Delete("1") {
		t.Error("Delete(\"1\") twice should report true then false")
	}
	if keys := s.Keys(); !slices.IsSorted(keys) {
		t.Errorf("Keys() not sorted: %q", keys)
	}
}

// kvStore is the subset of methods both implementations share.
type kvStore interface {
	Set(key, value string, ttl time.Duration)
	Get(key string) (string, error)
}

// benchMixed runs a 90% read / 10% write workload from GOMAXPROCS goroutines.
func benchMixed(b *testing.B, s kvStore) {
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = "key-" + strconv.Itoa(i)
		s.Set(keys[i], "value", 0)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			k := keys[i&1023]
			if i%10 == 0 {
				s.Set(k, "value", time.Minute)
			} else {
				_, _ = s.Get(k)
			}
			i++
		}
	})
}

func BenchmarkMixedStore(b *testing.B)   { benchMixed(b, New()) }
func BenchmarkMixedSharded(b *testing.B) { benchMixed(b, NewSharded(nil)) }
