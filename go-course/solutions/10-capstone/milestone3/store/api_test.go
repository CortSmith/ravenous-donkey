package store

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeClock is a manually advanced clock, safe for concurrent use.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

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

// openTestDB opens a DB at a fresh temp path with the given clock.
func openTestDB(t *testing.T, now func() time.Time) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.log")
	opts := Options{Now: now}
	db, err := Open(path, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// testServer creates a Server with a test DB and logger.
func testServer(t *testing.T) (*Server, *DB) {
	t.Helper()
	db := openTestDB(t, time.Now)
	lg := slog.New(slog.NewTextHandler(os.Stderr, nil))
	return NewServer(db, lg), db
}

func TestGetNotFound(t *testing.T) {
	srv, db := testServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/kv/missing", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /kv/missing code = %d, want 404", w.Code)
	}

	var resp errorResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == "" {
		t.Error("error message is empty")
	}
	_ = db // used indirectly
}

func TestGetAfterPut(t *testing.T) {
	srv, _ := testServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)

	// PUT /kv/key "value"
	putBody := putRequest{Value: "test-value"}
	putData, _ := json.Marshal(putBody)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/kv/test", bytes.NewReader(putData))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("PUT code = %d, want 200", w.Code)
	}

	// GET /kv/key
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/kv/test", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("GET code = %d, want 200", w.Code)
	}
	type getResp struct {
		Value string `json:"value"`
	}
	var resp getResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Value != "test-value" {
		t.Errorf("GET returned %q, want %q", resp.Value, "test-value")
	}
}

func TestPutWithTTL(t *testing.T) {
	clk := &fakeClock{now: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	db := openTestDB(t, clk.Now)
	lg := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := &Server{db: db, lg: lg}
	mux := http.NewServeMux()
	srv.Register(mux)

	// PUT with 1 second TTL
	putBody := putRequest{Value: "expires-soon", TTL: 1000} // 1000ms = 1s
	putData, _ := json.Marshal(putBody)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/kv/expiring", bytes.NewReader(putData))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT code = %d, want 200", w.Code)
	}

	// Should be readable immediately
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/kv/expiring", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("GET immediately: code = %d, want 200", w.Code)
	}

	// Advance time past expiry
	clk.Advance(2 * time.Second)

	// Should now be gone
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/kv/expiring", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("GET after expiry: code = %d, want 404", w.Code)
	}
}

func TestDeleteExisting(t *testing.T) {
	srv, _ := testServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)

	// PUT first
	putBody := putRequest{Value: "value"}
	putData, _ := json.Marshal(putBody)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/kv/key", bytes.NewReader(putData))
	mux.ServeHTTP(w, r)

	// DELETE
	w = httptest.NewRecorder()
	r = httptest.NewRequest("DELETE", "/kv/key", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("DELETE code = %d, want 200", w.Code)
	}

	// GET should return 404
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/kv/key", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("GET after DELETE: code = %d, want 404", w.Code)
	}
}

func TestDeleteMissing(t *testing.T) {
	srv, _ := testServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/kv/missing", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("DELETE /kv/missing code = %d, want 404", w.Code)
	}
}

func TestEmptyKey(t *testing.T) {
	srv, _ := testServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)

	// GET with empty key (the route requires {key}, so this won't match)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/kv/", nil)
	mux.ServeHTTP(w, r)

	// This will be a 404 because the pattern /kv/{key} doesn't match /kv/
	if w.Code != http.StatusNotFound {
		t.Errorf("GET /kv/ code = %d, want 404", w.Code)
	}
}

func TestPutInvalidBody(t *testing.T) {
	srv, _ := testServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/kv/key", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("PUT with invalid body: code = %d, want 400", w.Code)
	}
}
