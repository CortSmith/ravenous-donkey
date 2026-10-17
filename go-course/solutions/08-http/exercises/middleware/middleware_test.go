package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// newTestLogger returns a JSON logger writing to buf, at Debug level.
func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// logRecords decodes every JSON log line in buf.
func logRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var recs []map[string]any
	dec := json.NewDecoder(buf)
	for {
		var m map[string]any
		err := dec.Decode(&m)
		if errors.Is(err, io.EOF) {
			return recs
		}
		if err != nil {
			t.Fatalf("log output is not JSON lines: %v", err)
		}
		recs = append(recs, m)
	}
}

// tag returns middleware that appends name to the X-Trace header on the way
// in, so tests can see the order in which middleware ran.
func tag(name string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Trace", name)
			next.ServeHTTP(w, r)
		})
	}
}

func TestChain(t *testing.T) {
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Trace", "handler")
	})
	tests := []struct {
		name string
		mws  []Middleware
		want []string
	}{
		{"no middleware", nil, []string{"handler"}},
		{"one", []Middleware{tag("a")}, []string{"a", "handler"}},
		{"first is outermost", []Middleware{tag("a"), tag("b"), tag("c")}, []string{"a", "b", "c", "handler"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Chain(final, tt.mws...).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			if got := rec.Header().Values("X-Trace"); !slices.Equal(got, tt.want) {
				t.Errorf("middleware order = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	gen := func() string { return "generated-1" }
	tests := []struct {
		name     string
		incoming string
		want     string
	}{
		{"generated when absent", "", "generated-1"},
		{"incoming reused", "abc-123", "abc-123"},
		{"64 bytes accepted", strings.Repeat("x", 64), strings.Repeat("x", 64)},
		{"too long replaced", strings.Repeat("x", 65), "generated-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			var seenOK bool
			h := RequestID(gen)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen, seenOK = RequestIDFrom(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.incoming != "" {
				req.Header.Set("X-Request-ID", tt.incoming)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if !seenOK || seen != tt.want {
				t.Errorf("RequestIDFrom in handler = (%q, %v), want (%q, true)", seen, seenOK, tt.want)
			}
			if got := rec.Header().Get("X-Request-ID"); got != tt.want {
				t.Errorf("X-Request-ID response header = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequestIDFromEmptyContext(t *testing.T) {
	if id, ok := RequestIDFrom(t.Context()); ok || id != "" {
		t.Errorf("RequestIDFrom(empty ctx) = (%q, %v), want (\"\", false)", id, ok)
	}
}

func TestLogging(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus float64
		wantBytes  float64
	}{
		{
			name: "explicit status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
				io.WriteString(w, "short")
			},
			wantStatus: 418,
			wantBytes:  5,
		},
		{
			name:       "implicit 200 on write",
			handler:    func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "hello, world") },
			wantStatus: 200,
			wantBytes:  12,
		},
		{
			name:       "nothing written",
			handler:    func(w http.ResponseWriter, r *http.Request) {},
			wantStatus: 200,
			wantBytes:  0,
		},
		{
			name:       "multiple writes",
			handler:    func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ab"); io.WriteString(w, "cde") },
			wantStatus: 200,
			wantBytes:  5,
		},
		{
			name: "second WriteHeader ignored",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantStatus: 202,
			wantBytes:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := Logging(newTestLogger(&buf))(tt.handler)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/items/7?x=1", nil))

			if rec.Code != int(tt.wantStatus) {
				t.Errorf("client saw status %d, want %d (the wrapper must pass WriteHeader through)", rec.Code, int(tt.wantStatus))
			}
			recs := logRecords(t, &buf)
			if len(recs) != 1 {
				t.Fatalf("got %d log records, want exactly 1: %v", len(recs), recs)
			}
			got := recs[0]
			want := map[string]any{
				"level":  "INFO",
				"msg":    "request",
				"method": "POST",
				"path":   "/items/7",
				"status": tt.wantStatus,
				"bytes":  tt.wantBytes,
			}
			for k, v := range want {
				if got[k] != v {
					t.Errorf("log[%q] = %v (%T), want %v (%T)", k, got[k], got[k], v, v)
				}
			}
			if d, ok := got["duration"].(float64); !ok || d < 0 {
				t.Errorf("log[duration] = %v, want a non-negative number (pass a time.Duration)", got["duration"])
			}
			if _, ok := got["request_id"]; ok {
				t.Errorf("log has request_id %v but the request had none", got["request_id"])
			}
		})
	}
}

func TestLoggingIncludesRequestID(t *testing.T) {
	var buf bytes.Buffer
	h := Chain(http.NotFoundHandler(), RequestID(func() string { return "rid-9" }), Logging(newTestLogger(&buf)))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))

	recs := logRecords(t, &buf)
	if len(recs) != 1 {
		t.Fatalf("got %d log records, want 1", len(recs))
	}
	if recs[0]["request_id"] != "rid-9" {
		t.Errorf("log[request_id] = %v, want %q", recs[0]["request_id"], "rid-9")
	}
	if recs[0]["status"] != float64(404) {
		t.Errorf("log[status] = %v, want 404", recs[0]["status"])
	}
}

func TestLoggingPreservesFlusher(t *testing.T) {
	var buf bytes.Buffer
	h := Logging(newTestLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "chunk")
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush through Logging wrapper: %v (implement Unwrap() http.ResponseWriter)", err)
		}
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))
	if !rec.Flushed {
		t.Error("underlying recorder was not flushed")
	}
}

func TestRecover(t *testing.T) {
	t.Run("panic before writing gives 500", func(t *testing.T) {
		var buf bytes.Buffer
		h := Recover(newTestLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("kaboom")
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil)) // must not panic

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
		recs := logRecords(t, &buf)
		if len(recs) != 1 {
			t.Fatalf("got %d log records, want 1", len(recs))
		}
		if recs[0]["level"] != "ERROR" || recs[0]["msg"] != "panic recovered" {
			t.Errorf("log = %v, want level ERROR and msg %q", recs[0], "panic recovered")
		}
		if recs[0]["panic"] != "kaboom" {
			t.Errorf("log[panic] = %v, want %q", recs[0]["panic"], "kaboom")
		}
	})

	t.Run("panic with error value", func(t *testing.T) {
		var buf bytes.Buffer
		h := Recover(newTestLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(errors.New("nil map write"))
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
		if recs := logRecords(t, &buf); len(recs) != 1 || recs[0]["panic"] != "nil map write" {
			t.Errorf("log records = %v, want one with panic=%q", recs, "nil map write")
		}
	})

	t.Run("panic after writing keeps partial response", func(t *testing.T) {
		var buf bytes.Buffer
		h := Recover(newTestLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "partial")
			panic("late")
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (already sent before the panic)", rec.Code)
		}
		if got := rec.Body.String(); got != "partial" {
			t.Errorf("body = %q, want %q; do not write an error body after the response started", got, "partial")
		}
	})

	t.Run("no panic passes through", func(t *testing.T) {
		var buf bytes.Buffer
		h := Recover(newTestLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201", rec.Code)
		}
		if buf.Len() != 0 {
			t.Errorf("unexpected log output: %s", buf.String())
		}
	})

	t.Run("ErrAbortHandler is re-panicked", func(t *testing.T) {
		var buf bytes.Buffer
		h := Recover(newTestLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(http.ErrAbortHandler)
		}))
		defer func() {
			if v := recover(); v != http.ErrAbortHandler {
				t.Errorf("recovered %v, want http.ErrAbortHandler to propagate", v)
			}
		}()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		t.Error("ServeHTTP returned normally; http.ErrAbortHandler must be re-panicked")
	})
}

func TestFullStack(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)
	h := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("deep bug") }),
		RequestID(func() string { return "rid-1" }),
		Logging(logger),
		Recover(logger),
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if got := rec.Header().Get("X-Request-ID"); got != "rid-1" {
		t.Errorf("X-Request-ID = %q, want %q", got, "rid-1")
	}
	recs := logRecords(t, &buf)
	if len(recs) != 2 {
		t.Fatalf("got %d log records, want 2 (panic + access log): %v", len(recs), recs)
	}
	if recs[1]["msg"] != "request" || recs[1]["status"] != float64(500) || recs[1]["request_id"] != "rid-1" {
		t.Errorf("access log = %v, want msg=request status=500 request_id=rid-1", recs[1])
	}
}
