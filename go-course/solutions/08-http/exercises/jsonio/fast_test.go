package jsonio

import (
	"net/http"
	"testing"
)

// discardWriter is a minimal ResponseWriter so the benchmark measures the
// helper, not httptest.ResponseRecorder's own allocations.
type discardWriter struct{ h http.Header }

func (d *discardWriter) Header() http.Header         { return d.h }
func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }
func (d *discardWriter) WriteHeader(int)             {}

func TestWriteJSONFastMatches(t *testing.T) {
	for _, v := range []any{person{"Ann", 40}, map[string]int{"id": 7}, nil} {
		slow := &recordingWriter{h: http.Header{}}
		fast := &recordingWriter{h: http.Header{}}
		if err := WriteJSON(slow, http.StatusCreated, v); err != nil {
			t.Fatal(err)
		}
		if err := WriteJSONFast(fast, http.StatusCreated, v); err != nil {
			t.Fatal(err)
		}
		if slow.body != fast.body || slow.status != fast.status || slow.h.Get("Content-Type") != fast.h.Get("Content-Type") {
			t.Errorf("WriteJSONFast(%v) = (%d, %q), want (%d, %q)", v, fast.status, fast.body, slow.status, slow.body)
		}
	}
}

type recordingWriter struct {
	h      http.Header
	status int
	body   string
}

func (r *recordingWriter) Header() http.Header         { return r.h }
func (r *recordingWriter) Write(p []byte) (int, error) { r.body += string(p); return len(p), nil }
func (r *recordingWriter) WriteHeader(s int)           { r.status = s }

var benchValue = []person{{"Ann", 40}, {"Bo", 3}, {"Cy", 77}, {"Di", 12}}

func BenchmarkWriteJSON(b *testing.B) {
	w := &discardWriter{h: http.Header{}}
	b.ReportAllocs()
	for b.Loop() {
		_ = WriteJSON(w, http.StatusOK, benchValue)
	}
}

func BenchmarkWriteJSONFast(b *testing.B) {
	w := &discardWriter{h: http.Header{}}
	b.ReportAllocs()
	for b.Loop() {
		_ = WriteJSONFast(w, http.StatusOK, benchValue)
	}
}
