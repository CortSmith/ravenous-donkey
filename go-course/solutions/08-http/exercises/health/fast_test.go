package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandlerParallel(t *testing.T) {
	checks := map[string]Check{
		"a":    sleepCheck(100 * time.Millisecond),
		"b":    sleepCheck(100 * time.Millisecond),
		"c":    sleepCheck(100 * time.Millisecond),
		"down": func(context.Context) error { return errors.New("boom") },
	}
	start := time.Now()
	rec, got := serve(t, t.Context(), HandlerParallel(time.Second, checks))
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Errorf("parallel handler took %v, want close to one check (100ms), not the sum (300ms)", elapsed)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got.Checks["a"] != "ok" || got.Checks["down"] != "boom" {
		t.Errorf("checks = %v", got.Checks)
	}
}

// Three checks that each take 1ms: sequential ≈ 3ms/op, parallel ≈ 1ms/op.
func benchmarkHealth(b *testing.B, newHandler func(time.Duration, map[string]Check) http.Handler) {
	checks := map[string]Check{
		"db":    sleepCheck(time.Millisecond),
		"cache": sleepCheck(time.Millisecond),
		"queue": sleepCheck(time.Millisecond),
	}
	h := newHandler(time.Second, checks)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	for b.Loop() {
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkHandler(b *testing.B)         { benchmarkHealth(b, Handler) }
func BenchmarkHandlerParallel(b *testing.B) { benchmarkHealth(b, HandlerParallel) }
