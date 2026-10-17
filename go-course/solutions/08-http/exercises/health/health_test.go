package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

// sleepCheck returns a Check that takes d to succeed but gives up as soon as
// its context is done.
func sleepCheck(d time.Duration) Check {
	return func(ctx context.Context) error {
		select {
		case <-time.After(d):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// serve runs h for a GET /healthz built on ctx and decodes the JSON body.
func serve(t *testing.T, ctx context.Context, h http.Handler) (*httptest.ResponseRecorder, Response) {
	t.Helper()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body %q is not valid JSON: %v", rec.Body.String(), err)
	}
	return rec, body
}

func TestHandler(t *testing.T) {
	ok := func(context.Context) error { return nil }
	down := func(context.Context) error { return errors.New("connection refused") }

	tests := []struct {
		name       string
		checks     map[string]Check
		wantStatus int
		want       Response
	}{
		{
			name:       "no checks",
			checks:     nil,
			wantStatus: http.StatusOK,
			want:       Response{Status: "ok"},
		},
		{
			name:       "all healthy",
			checks:     map[string]Check{"db": ok, "cache": ok},
			wantStatus: http.StatusOK,
			want:       Response{Status: "ok", Checks: map[string]string{"db": "ok", "cache": "ok"}},
		},
		{
			name:       "one failing",
			checks:     map[string]Check{"db": ok, "cache": down},
			wantStatus: http.StatusServiceUnavailable,
			want:       Response{Status: "fail", Checks: map[string]string{"db": "ok", "cache": "connection refused"}},
		},
		{
			name:       "all failing",
			checks:     map[string]Check{"a": down, "b": down},
			wantStatus: http.StatusServiceUnavailable,
			want:       Response{Status: "fail", Checks: map[string]string{"a": "connection refused", "b": "connection refused"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, got := serve(t, t.Context(), Handler(time.Second, tt.checks))
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if len(got.Checks) == 0 {
				got.Checks = nil // treat {} and absent the same
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("body = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHandlerHeaders(t *testing.T) {
	rec, _ := serve(t, t.Context(), Handler(time.Second, nil))
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}

func TestHandlerTimeout(t *testing.T) {
	h := Handler(20*time.Millisecond, map[string]Check{"slow": sleepCheck(10 * time.Second)})

	start := time.Now()
	rec, got := serve(t, t.Context(), h)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("handler took %v; the check should have been cut off after ~20ms (did you use context.WithTimeout?)", elapsed)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if want := context.DeadlineExceeded.Error(); got.Checks["slow"] != want {
		t.Errorf("checks[slow] = %q, want %q", got.Checks["slow"], want)
	}
}

func TestHandlerTimeoutIsPerCheck(t *testing.T) {
	// Each check needs 60ms and the budget is 150ms. If the budget were shared
	// by all checks (one context for the whole loop) the later checks would
	// fail; with a fresh deadline per check they all pass.
	checks := map[string]Check{
		"a": sleepCheck(60 * time.Millisecond),
		"b": sleepCheck(60 * time.Millisecond),
		"c": sleepCheck(60 * time.Millisecond),
	}
	rec, got := serve(t, t.Context(), Handler(150*time.Millisecond, checks))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; checks = %v (is the timeout applied per check?)", rec.Code, http.StatusOK, got.Checks)
	}
}

type ctxKey struct{}

func TestHandlerUsesRequestContext(t *testing.T) {
	t.Run("values flow to checks", func(t *testing.T) {
		check := func(ctx context.Context) error {
			if ctx.Value(ctxKey{}) != "trace-123" {
				return errors.New("check context is not derived from the request context")
			}
			return nil
		}
		ctx := context.WithValue(t.Context(), ctxKey{}, "trace-123")
		rec, got := serve(t, ctx, Handler(time.Second, map[string]Check{"ctx": check}))
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d; checks = %v", rec.Code, http.StatusOK, got.Checks)
		}
	})

	t.Run("cancelled request cancels checks", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel() // the client has already gone away

		start := time.Now()
		_, got := serve(t, ctx, Handler(10*time.Second, map[string]Check{"slow": sleepCheck(10 * time.Second)}))
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("handler took %v after the request was cancelled", elapsed)
		}
		if want := context.Canceled.Error(); got.Checks["slow"] != want {
			t.Errorf("checks[slow] = %q, want %q", got.Checks["slow"], want)
		}
	})
}
