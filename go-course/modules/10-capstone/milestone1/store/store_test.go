package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
)

// t0 is an arbitrary fixed instant used as "now" at the start of each test.
var t0 = time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

// fakeClock is a manually advanced clock. It is safe for concurrent use
// because the janitor goroutine reads it while the test advances it.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{now: t0} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// newTestStore returns a Store driven by a fresh fake clock.
func newTestStore(t *testing.T) (*Store, *fakeClock) {
	t.Helper()
	clk := newFakeClock()
	s := New(WithClock(clk.Now))
	if s == nil {
		t.Fatal("New returned nil")
	}
	return s, clk
}

// mustGet fails the test unless key holds want.
func mustGet(t *testing.T, s *Store, key, want string) {
	t.Helper()
	got, err := s.Get(key)
	if err != nil {
		t.Fatalf("Get(%q) error = %v, want value %q", key, err, want)
	}
	if got != want {
		t.Fatalf("Get(%q) = %q, want %q", key, got, want)
	}
}

// mustMiss fails the test unless key is reported as not found.
func mustMiss(t *testing.T, s *Store, key string) {
	t.Helper()
	got, err := s.Get(key)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(%q) = (%q, %v), want error ErrNotFound", key, got, err)
	}
	if got != "" {
		t.Errorf("Get(%q) returned value %q alongside ErrNotFound, want empty string", key, got)
	}
}

func TestSetGet(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"simple", "a", "1"},
		{"empty value", "empty", ""},
		{"empty key", "", "value for empty key"},
		{"unicode", "ключ", "значение ✓"},
		{"spaces and newlines", "k e y", "line1\nline2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newTestStore(t)
			s.Set(tt.key, tt.value, 0)
			mustGet(t, s, tt.key, tt.value)
		})
	}
}

func TestGetMissing(t *testing.T) {
	s, _ := newTestStore(t)
	mustMiss(t, s, "nope")
}

func TestSetOverwrites(t *testing.T) {
	s, _ := newTestStore(t)
	s.Set("k", "v1", 0)
	s.Set("k", "v2", 0)
	mustGet(t, s, "k", "v2")
	if n := s.Len(); n != 1 {
		t.Errorf("Len() after overwriting one key = %d, want 1", n)
	}
}

func TestTTLExpiry(t *testing.T) {
	tests := []struct {
		name    string
		ttl     time.Duration
		advance time.Duration
		live    bool
	}{
		{"just set", 10 * time.Second, 0, true},
		{"one nanosecond before expiry", 10 * time.Second, 10*time.Second - time.Nanosecond, true},
		{"exactly at expiry", 10 * time.Second, 10 * time.Second, false},
		{"after expiry", 10 * time.Second, time.Hour, false},
		{"zero ttl never expires", 0, 1000 * time.Hour, true},
		{"negative ttl never expires", -time.Second, 1000 * time.Hour, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, clk := newTestStore(t)
			s.Set("k", "v", tt.ttl)
			clk.Advance(tt.advance)
			if tt.live {
				mustGet(t, s, "k", "v")
			} else {
				mustMiss(t, s, "k")
			}
		})
	}
}

func TestSetResetsTTL(t *testing.T) {
	t.Run("new ttl replaces old", func(t *testing.T) {
		s, clk := newTestStore(t)
		s.Set("k", "v1", 10*time.Second)
		clk.Advance(8 * time.Second)
		s.Set("k", "v2", 10*time.Second) // now expires at t0+18s
		clk.Advance(8 * time.Second)     // t0+16s
		mustGet(t, s, "k", "v2")
		clk.Advance(2 * time.Second) // t0+18s
		mustMiss(t, s, "k")
	})
	t.Run("zero ttl clears expiry", func(t *testing.T) {
		s, clk := newTestStore(t)
		s.Set("k", "v1", time.Second)
		s.Set("k", "v2", 0)
		clk.Advance(time.Hour)
		mustGet(t, s, "k", "v2")
	})
	t.Run("set after expiry revives key", func(t *testing.T) {
		s, clk := newTestStore(t)
		s.Set("k", "old", time.Second)
		clk.Advance(2 * time.Second)
		mustMiss(t, s, "k")
		s.Set("k", "new", 0)
		mustGet(t, s, "k", "new")
	})
}

func TestDelete(t *testing.T) {
	s, clk := newTestStore(t)
	s.Set("live", "1", 0)
	s.Set("short", "2", time.Second)
	clk.Advance(time.Second)

	tests := []struct {
		key  string
		want bool
	}{
		{"live", true},
		{"live", false}, // second delete: already gone
		{"short", false},
		{"missing", false},
	}
	for _, tt := range tests {
		if got := s.Delete(tt.key); got != tt.want {
			t.Errorf("Delete(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
	mustMiss(t, s, "live")
	if n := s.storedLen(); n != 0 {
		t.Errorf("after deleting everything the map still holds %d entries, want 0 (Delete should remove expired entries too)", n)
	}
}

func TestKeys(t *testing.T) {
	t.Run("empty store", func(t *testing.T) {
		s, _ := newTestStore(t)
		if got := s.Keys(); len(got) != 0 {
			t.Errorf("Keys() on empty store = %q, want empty", got)
		}
	})
	t.Run("sorted and live only", func(t *testing.T) {
		s, clk := newTestStore(t)
		for _, k := range []string{"pear", "apple", "fig", "banana"} {
			s.Set(k, "x", 0)
		}
		s.Set("expiring", "x", time.Minute)
		s.Set("later", "x", time.Hour)
		clk.Advance(2 * time.Minute)

		want := []string{"apple", "banana", "fig", "later", "pear"}
		if got := s.Keys(); !slices.Equal(got, want) {
			t.Errorf("Keys() = %q, want %q", got, want)
		}
	})
	t.Run("caller owns result", func(t *testing.T) {
		s, _ := newTestStore(t)
		s.Set("a", "1", 0)
		s.Set("b", "2", 0)
		keys := s.Keys()
		if len(keys) != 2 {
			t.Fatalf("Keys() = %q, want 2 keys", keys)
		}
		keys[0] = "mutated"
		if got := s.Keys(); !slices.Equal(got, []string{"a", "b"}) {
			t.Errorf("mutating the slice returned by Keys changed the store: Keys() = %q", got)
		}
	})
}

func TestLen(t *testing.T) {
	s, clk := newTestStore(t)
	if n := s.Len(); n != 0 {
		t.Fatalf("Len() of new store = %d, want 0", n)
	}
	s.Set("a", "1", 0)
	s.Set("b", "2", time.Second)
	s.Set("c", "3", time.Minute)
	if n := s.Len(); n != 3 {
		t.Errorf("Len() = %d, want 3", n)
	}
	clk.Advance(time.Second)
	if n := s.Len(); n != 2 {
		t.Errorf("Len() after one key expired = %d, want 2", n)
	}
}

func TestDeleteExpired(t *testing.T) {
	s, clk := newTestStore(t)
	s.Set("forever", "x", 0)
	for i := range 5 {
		s.Set(fmt.Sprintf("short%d", i), "x", time.Second)
	}
	s.Set("long", "x", time.Hour)

	if n := s.DeleteExpired(); n != 0 {
		t.Errorf("DeleteExpired() before anything expired = %d, want 0", n)
	}
	clk.Advance(time.Second)
	if n := s.DeleteExpired(); n != 5 {
		t.Errorf("DeleteExpired() = %d, want 5", n)
	}
	if n := s.storedLen(); n != 2 {
		t.Errorf("map holds %d entries after DeleteExpired, want 2", n)
	}
	if n := s.DeleteExpired(); n != 0 {
		t.Errorf("second DeleteExpired() = %d, want 0", n)
	}
	mustGet(t, s, "forever", "x")
	mustGet(t, s, "long", "x")
}

// sendTick delivers one tick or fails the test if the janitor is not
// receiving.
func sendTick(t *testing.T, ticks chan<- time.Time) {
	t.Helper()
	select {
	case ticks <- time.Time{}:
	case <-time.After(2 * time.Second):
		t.Fatal("janitor did not receive a tick within 2s (is it running a receive loop?)")
	}
}

// waitDone fails the test unless done is closed within 2s.
func waitDone(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not return within 2s of context cancellation", what)
	}
}

func TestJanitorSweepsOnTick(t *testing.T) {
	s, clk := newTestStore(t)
	s.Set("a", "1", time.Second)
	s.Set("b", "2", time.Second)
	s.Set("keep", "3", 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time) // unbuffered: a send completes only when the janitor receives
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.janitor(ctx, ticks)
	}()

	// Nothing has expired yet: a sweep must not remove anything.
	sendTick(t, ticks)
	sendTick(t, ticks) // the janitor is sequential, so this send proves the first sweep finished
	if n := s.storedLen(); n != 3 {
		t.Fatalf("map holds %d entries after a sweep with nothing expired, want 3", n)
	}

	clk.Advance(time.Second)
	sendTick(t, ticks)
	sendTick(t, ticks)
	if n := s.storedLen(); n != 1 {
		t.Fatalf("map holds %d entries after sweeping 2 expired keys, want 1", n)
	}

	cancel()
	waitDone(t, done, "janitor")
}

func TestJanitorStopsOnCancel(t *testing.T) {
	s, _ := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.janitor(ctx, make(chan time.Time)) // never ticks
	}()
	cancel()
	waitDone(t, done, "janitor")
}

func TestRunJanitor(t *testing.T) {
	s, clk := newTestStore(t)
	s.Set("gone", "x", time.Second)
	s.Set("keep", "x", 0)
	clk.Advance(time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.RunJanitor(ctx, time.Millisecond)
	}()

	// A real ticker is involved here, so poll (with a deadline) rather than
	// assuming how many ticks have happened.
	deadline := time.Now().Add(2 * time.Second)
	for s.storedLen() != 1 {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("RunJanitor(ctx, 1ms) did not remove the expired key within 2s; map holds %d entries", s.storedLen())
		}
		time.Sleep(time.Millisecond)
	}
	mustGet(t, s, "keep", "x")

	cancel()
	waitDone(t, done, "RunJanitor")
}

func TestRunJanitorDisabled(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			s, _ := newTestStore(t)
			done := make(chan struct{})
			go func() {
				defer close(done)
				s.RunJanitor(context.Background(), interval) // never canceled
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("RunJanitor(ctx, %v) blocked; want immediate return for non-positive interval", interval)
			}
		})
	}
}

func TestDefaultClock(t *testing.T) {
	s := New()
	s.Set("k", "v", time.Hour)
	mustGet(t, s, "k", "v")
	s.Set("gone", "v", time.Nanosecond)
	time.Sleep(time.Millisecond) // the one place we must sleep: real clock
	mustMiss(t, s, "gone")
}

// TestConcurrentAccess hammers the store from many goroutines while the
// janitor runs. It finds bugs only under the race detector:
//
//	go test -race ./...
func TestConcurrentAccess(t *testing.T) {
	s, clk := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	janitorDone := make(chan struct{})
	go func() {
		defer close(janitorDone)
		s.RunJanitor(ctx, time.Millisecond)
	}()

	const workers, ops = 8, 500
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ops {
				key := fmt.Sprintf("k%d", (w*ops+i)%50)
				switch i % 6 {
				case 0:
					s.Set(key, "v", time.Duration(i%3)*time.Millisecond)
				case 1:
					_, _ = s.Get(key)
				case 2:
					s.Delete(key)
				case 3:
					_ = s.Keys()
				case 4:
					_ = s.Len()
				case 5:
					clk.Advance(time.Millisecond)
				}
			}
		}()
	}
	wg.Wait()
	cancel()
	waitDone(t, janitorDone, "RunJanitor")

	// Every remaining key must be readable and Keys/Len must agree.
	keys := s.Keys()
	if len(keys) != s.Len() {
		t.Errorf("len(Keys()) = %d but Len() = %d", len(keys), s.Len())
	}
	for _, k := range keys {
		mustGet(t, s, k, "v")
	}
}
