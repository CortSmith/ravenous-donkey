package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newServer starts an httptest server for h and closes it when the test ends.
func newServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// blockingServer answers only after the test ends (or the client gives up),
// simulating a hung upstream.
func blockingServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	// Cleanups run last-in first-out: release the handler before srv.Close,
	// which waits for in-flight requests.
	t.Cleanup(func() { close(release) })
	return srv
}

func TestGetItem(t *testing.T) {
	var gotPath, gotAccept, gotMethod string
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAccept = r.Method, r.URL.EscapedPath(), r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"42","name":"Widget","price":1999}`)
	})

	got, err := New(srv.URL, time.Second).GetItem(t.Context(), "42")
	if err != nil {
		t.Fatalf("GetItem error = %v", err)
	}
	if want := (Item{ID: "42", Name: "Widget", Price: 1999}); got != want {
		t.Errorf("GetItem = %+v, want %+v", got, want)
	}
	if gotMethod != http.MethodGet || gotPath != "/items/42" {
		t.Errorf("request = %s %s, want GET /items/42", gotMethod, gotPath)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept header = %q, want %q", gotAccept, "application/json")
	}
}

func TestGetItemEscapesID(t *testing.T) {
	tests := []struct{ id, wantPath string }{
		{"a/b", "/items/a%2Fb"},
		{"hello world", "/items/hello%20world"},
		{"x?y=1", "/items/x%3Fy=1"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			var gotPath string
			srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.EscapedPath()
				io.WriteString(w, `{}`)
			})
			if _, err := New(srv.URL, time.Second).GetItem(t.Context(), tt.id); err != nil {
				t.Fatalf("GetItem(%q) error = %v", tt.id, err)
			}
			if gotPath != tt.wantPath {
				t.Errorf("GetItem(%q) requested path %q, want %q (use url.PathEscape)", tt.id, gotPath, tt.wantPath)
			}
		})
	}
}

func TestNewTrimsTrailingSlash(t *testing.T) {
	var gotPath string
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		io.WriteString(w, `{}`)
	})
	if _, err := New(srv.URL+"/", time.Second).GetItem(t.Context(), "1"); err != nil {
		t.Fatalf("GetItem error = %v", err)
	}
	if gotPath != "/items/1" {
		t.Errorf("path = %q, want %q", gotPath, "/items/1")
	}
}

func TestGetItemEmptyID(t *testing.T) {
	var hits atomic.Int32
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) { hits.Add(1) })
	if _, err := New(srv.URL, time.Second).GetItem(t.Context(), ""); err == nil {
		t.Error("GetItem(\"\") error = nil, want an error")
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("server received %d requests; an empty id should fail before any request", n)
	}
}

func TestGetItemErrors(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantNF   bool   // errors.Is(err, ErrNotFound)
		wantCode int    // >0: want *StatusError with this code
		wantBody string // StatusError.Body
	}{
		{name: "not found", status: 404, body: `{"error":"nope"}`, wantNF: true},
		{name: "server error", status: 500, body: "  database down\n", wantCode: 500, wantBody: "database down"},
		{name: "bad gateway empty", status: 502, body: "", wantCode: 502, wantBody: ""},
		{name: "unexpected 2xx", status: 204, body: "", wantCode: 204, wantBody: ""},
		{name: "long body truncated", status: 503, body: strings.Repeat("x", 2000), wantCode: 503, wantBody: strings.Repeat("x", 512)},
		{name: "malformed JSON", status: 200, body: `{"id":`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				io.WriteString(w, tt.body)
			})
			_, err := New(srv.URL, time.Second).GetItem(t.Context(), "7")
			if err == nil {
				t.Fatal("GetItem error = nil, want an error")
			}
			if !strings.Contains(err.Error(), `"7"`) {
				t.Errorf("error %q should mention the item id %q", err, "7")
			}
			if got := errors.Is(err, ErrNotFound); got != tt.wantNF {
				t.Errorf("errors.Is(err, ErrNotFound) = %v, want %v (err: %v)", got, tt.wantNF, err)
			}
			var se *StatusError
			isSE := errors.As(err, &se)
			if tt.wantCode == 0 {
				if isSE && !tt.wantNF {
					t.Errorf("got *StatusError %+v, want a different error", se)
				}
				return
			}
			if !isSE {
				t.Fatalf("error = %v (%T), want it to wrap *StatusError", err, err)
			}
			if se.Code != tt.wantCode || se.Body != tt.wantBody {
				t.Errorf("StatusError = {Code: %d, Body: %q}, want {Code: %d, Body: %q}", se.Code, se.Body, tt.wantCode, tt.wantBody)
			}
		})
	}
}

func TestStatusErrorMessage(t *testing.T) {
	err := &StatusError{Code: 500, Body: "oops"}
	if got, want := err.Error(), "unexpected status 500: oops"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestClientTimeout(t *testing.T) {
	srv := blockingServer(t)
	c := New(srv.URL, 50*time.Millisecond)

	start := time.Now()
	_, err := c.GetItem(t.Context(), "1") // the context has no deadline: only the client timeout applies
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("GetItem took %v; New must configure http.Client.Timeout", elapsed)
	}
	if err == nil {
		t.Fatal("GetItem error = nil, want a timeout error")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("error = %v, want a net.Error with Timeout() == true", err)
	}
}

func TestContextCancellation(t *testing.T) {
	t.Run("deadline", func(t *testing.T) {
		srv := blockingServer(t)
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		_, err := New(srv.URL, time.Minute).GetItem(ctx, "1")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error = %v, want errors.Is(err, context.DeadlineExceeded) (use http.NewRequestWithContext)", err)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		srv := blockingServer(t)
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(50*time.Millisecond, cancel)
		_, err := New(srv.URL, time.Minute).GetItem(ctx, "1")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want errors.Is(err, context.Canceled)", err)
		}
	})
}

func TestConnectionReuse(t *testing.T) {
	// Error responses with bodies the client does not need must still be
	// drained, or every request opens a new TCP connection.
	var hits atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1)%2 == 0 {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		io.WriteString(w, strings.Repeat("e", 4096))
	}))
	var newConns atomic.Int32
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			newConns.Add(1)
		}
	}
	srv.Start()
	t.Cleanup(srv.Close)

	c := New(srv.URL, time.Second)
	const requests = 10
	for range requests {
		if _, err := c.GetItem(t.Context(), "x"); err == nil {
			t.Fatal("GetItem error = nil, want an error")
		}
	}
	// A tiny allowance for scheduling; without draining it would be 10.
	if n := newConns.Load(); n > 2 {
		t.Errorf("%d requests opened %d connections; drain and close the response body so connections are reused", requests, n)
	}
}

func TestCreateItem(t *testing.T) {
	var (
		gotMethod, gotPath, gotCT string
		gotBody                   Item
	)
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotCT = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		created := gotBody
		created.ID = "new-1"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	})

	in := Item{Name: "Gadget", Price: 500}
	got, err := New(srv.URL, time.Second).CreateItem(t.Context(), in)
	if err != nil {
		t.Fatalf("CreateItem error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/items" {
		t.Errorf("request = %s %s, want POST /items", gotMethod, gotPath)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotCT, "application/json")
	}
	if gotBody != in {
		t.Errorf("server received %+v, want %+v", gotBody, in)
	}
	if want := (Item{ID: "new-1", Name: "Gadget", Price: 500}); got != want {
		t.Errorf("CreateItem = %+v, want %+v", got, want)
	}
}

func TestCreateItemStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"200 is not 201", http.StatusOK},
		{"validation error", http.StatusBadRequest},
		{"conflict", http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				io.WriteString(w, `{"id":"x"}`)
			})
			_, err := New(srv.URL, time.Second).CreateItem(t.Context(), Item{Name: "n"})
			var se *StatusError
			if !errors.As(err, &se) || se.Code != tt.status {
				t.Errorf("CreateItem error = %v, want *StatusError with Code %d", err, tt.status)
			}
		})
	}
}

func TestConcurrentUse(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":"1"}`)
	})
	c := New(srv.URL, time.Second)
	errs := make(chan error, 20)
	for range 20 {
		go func() {
			_, err := c.GetItem(t.Context(), "1")
			errs <- err
		}()
	}
	for range 20 {
		if err := <-errs; err != nil {
			t.Errorf("concurrent GetItem error = %v", err)
		}
	}
}
