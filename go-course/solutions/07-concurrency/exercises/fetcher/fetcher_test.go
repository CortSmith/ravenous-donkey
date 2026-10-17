package fetcher

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const timeout = 5 * time.Second

// fetchAllWithTimeout runs FetchAll on another goroutine so a deadlocked
// implementation fails the test instead of hanging it.
func fetchAllWithTimeout(t *testing.T, ctx context.Context, urls []string, limit int, fetch FetchFunc) []Result {
	t.Helper()
	ch := make(chan []Result, 1)
	go func() { ch <- FetchAll(ctx, urls, limit, fetch) }()
	select {
	case r := <-ch:
		return r
	case <-time.After(timeout):
		t.Fatalf("FetchAll did not return within %v (semaphore never released? missing wg.Done?)", timeout)
		return nil
	}
}

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

func urls(n int) []string {
	u := make([]string, n)
	for i := range u {
		u[i] = fmt.Sprintf("https://example.com/%d", i)
	}
	return u
}

// echo is a fake fetch that returns "body of <url>" after a short delay that
// is longest for the earliest URLs, so completion order != input order.
func echo(n int) FetchFunc {
	return func(ctx context.Context, url string) (string, error) {
		var i int
		fmt.Sscanf(url, "https://example.com/%d", &i)
		select {
		case <-time.After(time.Duration(n-i) * 200 * time.Microsecond):
		case <-ctx.Done():
			return "", ctx.Err()
		}
		return "body of " + url, nil
	}
}

func TestFetchAllOrderAndBodies(t *testing.T) {
	tests := []struct {
		name  string
		n     int
		limit int
	}{
		{"empty", 0, 3},
		{"one", 1, 3},
		{"limit 1", 10, 1},
		{"limit 4", 25, 4},
		{"limit above n", 5, 100},
		{"limit 0 means 1", 4, 0},
		{"negative limit means 1", 4, -3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := runtime.NumGoroutine()
			in := urls(tt.n)
			got := fetchAllWithTimeout(t, t.Context(), in, tt.limit, echo(tt.n))
			if len(got) != tt.n {
				t.Fatalf("FetchAll returned %d results for %d URLs, want %d", len(got), tt.n, tt.n)
			}
			for i, r := range got {
				if r.URL != in[i] {
					t.Errorf("results[%d].URL = %q, want %q (results must be in input order)", i, r.URL, in[i])
				}
				if r.Err != nil {
					t.Errorf("results[%d].Err = %v, want nil", i, r.Err)
				}
				if want := "body of " + in[i]; r.Body != want {
					t.Errorf("results[%d].Body = %q, want %q", i, r.Body, want)
				}
			}
			checkNoLeak(t, base)
		})
	}
}

func TestFetchAllBoundsInFlight(t *testing.T) {
	for _, limit := range []int{1, 2, 5} {
		t.Run(fmt.Sprintf("limit=%d", limit), func(t *testing.T) {
			var inFlight, peak atomic.Int32
			full := make(chan struct{})
			var once sync.Once
			fetch := func(ctx context.Context, url string) (string, error) {
				n := inFlight.Add(1)
				defer inFlight.Add(-1)
				for {
					p := peak.Load()
					if n <= p || peak.CompareAndSwap(p, n) {
						break
					}
				}
				if int(n) >= limit {
					once.Do(func() { close(full) })
				}
				// Hold the slot until the pool is full (or give up), so the
				// peak reflects the real bound rather than scheduling luck.
				select {
				case <-full:
				case <-time.After(300 * time.Millisecond):
				}
				time.Sleep(time.Millisecond)
				return url, nil
			}
			fetchAllWithTimeout(t, t.Context(), urls(limit*5), limit, fetch)
			if got := int(peak.Load()); got > limit {
				t.Errorf("peak in-flight fetches = %d, want <= %d", got, limit)
			} else if got < limit {
				t.Errorf("peak in-flight fetches = %d, want %d (fetches are not running concurrently)", got, limit)
			}
		})
	}
}

func TestFetchAllBoundsGoroutines(t *testing.T) {
	base := runtime.NumGoroutine()
	release := make(chan struct{})
	fetch := func(ctx context.Context, url string) (string, error) {
		<-release
		return url, nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		FetchAll(t.Context(), urls(500), 3, fetch)
	}()
	time.Sleep(50 * time.Millisecond)
	g := runtime.NumGoroutine() - base
	close(release)
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("FetchAll did not return")
	}
	// 3 fetch goroutines + the FetchAll caller + a little slack. Spawning one
	// goroutine per URL up front and blocking them on the semaphore would
	// show up here as ~500.
	if g > 10 {
		t.Errorf("%d extra goroutines while 3 fetches were blocked, want <= 10 (acquire the semaphore before starting each goroutine)", g)
	}
}

var errNotFound = errors.New("404 not found")

func TestFetchAllPartialFailure(t *testing.T) {
	in := urls(6)
	fetch := func(ctx context.Context, url string) (string, error) {
		if strings.HasSuffix(url, "/2") || strings.HasSuffix(url, "/4") {
			return "", errNotFound
		}
		return "ok", nil
	}
	got := fetchAllWithTimeout(t, t.Context(), in, 2, fetch)
	for i, r := range got {
		failed := i == 2 || i == 4
		switch {
		case failed && !errors.Is(r.Err, errNotFound):
			t.Errorf("results[%d].Err = %v, want errNotFound", i, r.Err)
		case !failed && (r.Err != nil || r.Body != "ok"):
			t.Errorf("results[%d] = {Body: %q, Err: %v}, want {Body: \"ok\", Err: nil} (one failure must not stop the others)", i, r.Body, r.Err)
		}
	}

	err := Errors(got)
	if !errors.Is(err, errNotFound) {
		t.Fatalf("Errors(results) = %v, want it to wrap errNotFound", err)
	}
	for _, u := range []string{in[2], in[4]} {
		if !strings.Contains(err.Error(), u) {
			t.Errorf("Errors(results) = %q, want it to mention %q", err, u)
		}
	}
	if n := strings.Count(err.Error(), "\n") + 1; n != 2 {
		t.Errorf("Errors(results) has %d lines, want 2 (one per failure, joined with errors.Join):\n%s", n, err)
	}
}

func TestErrorsAllOK(t *testing.T) {
	if err := Errors([]Result{{URL: "a", Body: "x"}, {URL: "b"}}); err != nil {
		t.Errorf("Errors(all ok) = %v, want nil", err)
	}
	if err := Errors(nil); err != nil {
		t.Errorf("Errors(nil) = %v, want nil", err)
	}
}

func TestFetchAllCanceled(t *testing.T) {
	t.Run("before start", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		var calls atomic.Int32
		got := fetchAllWithTimeout(t, ctx, urls(5), 2, func(ctx context.Context, url string) (string, error) {
			calls.Add(1)
			return "x", nil
		})
		if c := calls.Load(); c != 0 {
			t.Errorf("fetch called %d times with an already-canceled ctx, want 0", c)
		}
		if len(got) != 5 {
			t.Fatalf("got %d results, want 5 (one per URL even when canceled)", len(got))
		}
		for i, r := range got {
			if !errors.Is(r.Err, context.Canceled) {
				t.Errorf("results[%d].Err = %v, want context.Canceled", i, r.Err)
			}
			if r.URL != urls(5)[i] {
				t.Errorf("results[%d].URL = %q, want %q", i, r.URL, urls(5)[i])
			}
		}
	})
	t.Run("midway", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var calls atomic.Int32
		in := urls(10)
		got := fetchAllWithTimeout(t, ctx, in, 1, func(_ context.Context, url string) (string, error) {
			calls.Add(1)
			cancel() // the first fetch cancels everything, then succeeds
			return "first", nil
		})
		if c := calls.Load(); c != 1 {
			t.Errorf("fetch called %d times, want 1 (check ctx after acquiring the semaphore: select picks randomly among ready cases)", c)
		}
		if got[0].Err != nil || got[0].Body != "first" {
			t.Errorf("results[0] = {Body: %q, Err: %v}, want the completed fetch's result", got[0].Body, got[0].Err)
		}
		for i := 1; i < len(got); i++ {
			if !errors.Is(got[i].Err, context.Canceled) || got[i].URL != in[i] {
				t.Errorf("results[%d] = %+v, want URL %q with context.Canceled", i, got[i], in[i])
			}
		}
		checkNoLeak(t, base)
	})
	t.Run("in-flight fetches see cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		got := fetchAllWithTimeout(t, ctx, urls(4), 4, func(ctx context.Context, url string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(10 * time.Second):
				return "too slow", nil
			}
		})
		for i, r := range got {
			if !errors.Is(r.Err, context.DeadlineExceeded) {
				t.Errorf("results[%d].Err = %v, want context.DeadlineExceeded (pass ctx through to fetch)", i, r.Err)
			}
		}
	})
}
