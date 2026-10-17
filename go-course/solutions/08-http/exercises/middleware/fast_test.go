package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggingFastMatches(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		io.WriteString(w, "tea")
	})
	var buf bytes.Buffer
	Chain(h, RequestID(func() string { return "r1" }), LoggingFast(newTestLogger(&buf))).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/pot", nil))
	recs := logRecords(t, &buf)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	r := recs[0]
	if r["msg"] != "request" || r["status"] != float64(418) || r["bytes"] != float64(3) || r["path"] != "/pot" || r["request_id"] != "r1" {
		t.Errorf("record = %v", r)
	}
}

// nopWriter keeps the benchmark focused on the middleware itself.
type nopWriter struct{ h http.Header }

func (w *nopWriter) Header() http.Header         { return w.h }
func (w *nopWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *nopWriter) WriteHeader(int)             {}

func benchmarkLogging(b *testing.B, mw func(*slog.Logger) Middleware, level slog.Level) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: level}))
	h := mw(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	w := &nopWriter{h: http.Header{}}
	req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
	b.ReportAllocs()
	for b.Loop() {
		h.ServeHTTP(w, req)
	}
}

func BenchmarkLogging(b *testing.B)     { benchmarkLogging(b, Logging, slog.LevelInfo) }
func BenchmarkLoggingFast(b *testing.B) { benchmarkLogging(b, LoggingFast, slog.LevelInfo) }

// With Info disabled, LoggingFast skips the wrapper entirely.
func BenchmarkLoggingDisabled(b *testing.B)     { benchmarkLogging(b, Logging, slog.LevelWarn) }
func BenchmarkLoggingFastDisabled(b *testing.B) { benchmarkLogging(b, LoggingFast, slog.LevelWarn) }
