package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
)

// do sends one request through Routes(s) and returns the recorder.
func do(t *testing.T, s *Store, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	rec := httptest.NewRecorder()
	Routes(s).ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("body %q is not valid JSON for %T: %v", rec.Body.String(), v, err)
	}
	return v
}

// seeded returns a store containing notes 1 "first", 2 "second", 3 "third".
func seeded() *Store {
	s := NewStore()
	s.Add("first")
	s.Add("second")
	s.Add("third")
	return s
}

func TestIndex(t *testing.T) {
	rec := do(t, NewStore(), http.MethodGet, "/", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "notes API\n" {
		t.Errorf("GET / = %d %q, want 200 %q", rec.Code, rec.Body.String(), "notes API\n")
	}
}

func TestUnknownPathsAre404(t *testing.T) {
	// "/" registered without {$} would match every path. These must all 404.
	for _, path := range []string{"/nope", "/notes/1/extra", "/notes/", "/index.html"} {
		t.Run(path, func(t *testing.T) {
			if rec := do(t, seeded(), http.MethodGet, path, ""); rec.Code != http.StatusNotFound {
				t.Errorf("GET %s = %d, want 404 (body %q)", path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestList(t *testing.T) {
	t.Run("empty is [] not null", func(t *testing.T) {
		rec := do(t, NewStore(), http.MethodGet, "/notes", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
			t.Errorf("body = %q, want []", got)
		}
	})
	t.Run("ordered by id", func(t *testing.T) {
		rec := do(t, seeded(), http.MethodGet, "/notes", "")
		got := decode[[]Note](t, rec)
		want := []Note{{1, "first"}, {2, "second"}, {3, "third"}}
		if !slices.Equal(got, want) {
			t.Errorf("GET /notes = %v, want %v", got, want)
		}
	})
}

func TestCreate(t *testing.T) {
	s := seeded()
	rec := do(t, s, http.MethodPost, "/notes", `{"text":"  buy milk  "}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /notes = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	got := decode[Note](t, rec)
	if want := (Note{ID: 4, Text: "buy milk"}); got != want {
		t.Errorf("created note = %+v, want %+v (text should be trimmed)", got, want)
	}
	if loc := rec.Header().Get("Location"); loc != "/notes/4" {
		t.Errorf("Location = %q, want %q", loc, "/notes/4")
	}
	if n, ok := s.Get(4); !ok || n.Text != "buy milk" {
		t.Errorf("store.Get(4) = %+v, %v; note was not stored", n, ok)
	}
}

func TestCreateInvalid(t *testing.T) {
	tests := []struct{ name, body string }{
		{"empty text", `{"text":""}`},
		{"blank text", `{"text":"   "}`},
		{"missing text", `{}`},
		{"malformed", `{"text":`},
		{"wrong type", `{"text":42}`},
		{"no body", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			rec := do(t, s, http.MethodPost, "/notes", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("POST /notes %q = %d, want 400", tt.body, rec.Code)
			}
			if e := decode[map[string]string](t, rec); e["error"] == "" {
				t.Errorf("error body = %v, want {\"error\": ...}", e)
			}
			if len(s.List()) != 0 {
				t.Errorf("invalid request stored a note: %v", s.List())
			}
		})
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		path       string
		wantStatus int
		want       Note
	}{
		{"/notes/2", http.StatusOK, Note{2, "second"}},
		{"/notes/99", http.StatusNotFound, Note{}},
		{"/notes/abc", http.StatusBadRequest, Note{}},
		{"/notes/0", http.StatusBadRequest, Note{}},
		{"/notes/-1", http.StatusBadRequest, Note{}},
		{"/notes/1.5", http.StatusBadRequest, Note{}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := do(t, seeded(), http.MethodGet, tt.path, "")
			if rec.Code != tt.wantStatus {
				t.Fatalf("GET %s = %d, want %d (body %q)", tt.path, rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				if e := decode[map[string]string](t, rec); e["error"] == "" {
					t.Errorf("error body = %v, want {\"error\": ...}", e)
				}
				return
			}
			if got := decode[Note](t, rec); got != tt.want {
				t.Errorf("GET %s = %+v, want %+v", tt.path, got, tt.want)
			}
		})
	}
}

func TestLatest(t *testing.T) {
	t.Run("specific pattern beats wildcard", func(t *testing.T) {
		rec := do(t, seeded(), http.MethodGet, "/notes/latest", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /notes/latest = %d, want 200 (body %q)", rec.Code, rec.Body.String())
		}
		if got := decode[Note](t, rec); got != (Note{3, "third"}) {
			t.Errorf("latest = %+v, want {3 third}", got)
		}
	})
	t.Run("none yet", func(t *testing.T) {
		if rec := do(t, NewStore(), http.MethodGet, "/notes/latest", ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET /notes/latest on empty store = %d, want 404", rec.Code)
		}
	})
}

func TestDelete(t *testing.T) {
	s := seeded()
	rec := do(t, s, http.MethodDelete, "/notes/2", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /notes/2 = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 response has body %q, want none", rec.Body.String())
	}
	if _, ok := s.Get(2); ok {
		t.Error("note 2 still exists after DELETE")
	}
	if rec := do(t, s, http.MethodDelete, "/notes/2", ""); rec.Code != http.StatusNotFound {
		t.Errorf("second DELETE /notes/2 = %d, want 404", rec.Code)
	}
	if rec := do(t, s, http.MethodDelete, "/notes/x", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("DELETE /notes/x = %d, want 400", rec.Code)
	}
}

func TestFiles(t *testing.T) {
	tests := []struct{ path, want string }{
		{"/files/readme.txt", "file: readme.txt\n"},
		{"/files/a/b/c.go", "file: a/b/c.go\n"},
		{"/files/", "file: \n"}, // {path...} may match the empty remainder
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := do(t, NewStore(), http.MethodGet, tt.path, "")
			if rec.Code != http.StatusOK || rec.Body.String() != tt.want {
				t.Errorf("GET %s = %d %q, want 200 %q", tt.path, rec.Code, rec.Body.String(), tt.want)
			}
		})
	}
}

func TestFilesWithoutSlashRedirects(t *testing.T) {
	// Because "/files/{path...}" also matches "/files/", ServeMux redirects
	// "/files" there, just as it does for a plain "/files/" subtree pattern.
	rec := do(t, NewStore(), http.MethodGet, "/files", "")
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/files/" {
		t.Errorf("GET /files = %d Location %q, want 301 to /files/", rec.Code, rec.Header().Get("Location"))
	}
}

func TestMethodNotAllowed(t *testing.T) {
	tests := []struct {
		method, path string
		wantAllow    []string
	}{
		{http.MethodPut, "/notes/1", []string{"DELETE", "GET", "HEAD"}},
		{http.MethodDelete, "/notes", []string{"GET", "HEAD", "POST"}},
		{http.MethodPost, "/files/x", []string{"GET", "HEAD"}},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := do(t, seeded(), tt.method, tt.path, "")
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s = %d, want 405 (register routes with a method, e.g. \"GET /notes\")", tt.method, tt.path, rec.Code)
			}
			allow := strings.Split(rec.Header().Get("Allow"), ", ")
			slices.Sort(allow)
			if !slices.Equal(allow, tt.wantAllow) {
				t.Errorf("Allow = %v, want %v", allow, tt.wantAllow)
			}
		})
	}
}

func TestHeadMatchesGet(t *testing.T) {
	if rec := do(t, seeded(), http.MethodHead, "/notes/1", ""); rec.Code != http.StatusOK {
		t.Errorf("HEAD /notes/1 = %d, want 200", rec.Code)
	}
}

func TestConcurrentCreates(t *testing.T) {
	s := NewStore()
	h := Routes(s)
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(`{"text":"x"}`))
			h.ServeHTTP(httptest.NewRecorder(), req)
		}()
	}
	wg.Wait()
	if n := len(s.List()); n != 50 {
		t.Errorf("after 50 concurrent creates the store has %d notes, want 50", n)
	}
}
