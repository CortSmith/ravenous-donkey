package shortener

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMemoryStoreCreateGet(t *testing.T) {
	s := NewMemoryStore()
	ctx := t.Context()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	l := Link{Code: "abcd", URL: "https://go.dev", CreatedAt: created}

	if err := s.Create(ctx, l); err != nil {
		t.Fatalf("Create error = %v", err)
	}
	got, err := s.Get(ctx, "abcd")
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}
	if got != l {
		t.Errorf("Get = %+v, want %+v", got, l)
	}
	if err := s.Create(ctx, Link{Code: "abcd", URL: "https://example.com"}); !errors.Is(err, ErrExists) {
		t.Errorf("duplicate Create error = %v, want ErrExists", err)
	}
	if got, _ := s.Get(ctx, "abcd"); got.URL != "https://go.dev" {
		t.Errorf("duplicate Create overwrote the link: %+v", got)
	}
}

func TestMemoryStoreNotFound(t *testing.T) {
	s := NewMemoryStore()
	if _, err := s.Get(t.Context(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
	if _, err := s.Visit(t.Context(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Visit(missing) error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreVisit(t *testing.T) {
	s := NewMemoryStore()
	ctx := t.Context()
	if err := s.Create(ctx, Link{Code: "abcd", URL: "https://go.dev"}); err != nil {
		t.Fatal(err)
	}
	for want := int64(1); want <= 3; want++ {
		l, err := s.Visit(ctx, "abcd")
		if err != nil {
			t.Fatalf("Visit error = %v", err)
		}
		if l.Visits != want {
			t.Errorf("Visit #%d returned Visits = %d, want %d", want, l.Visits, want)
		}
	}
	if l, _ := s.Get(ctx, "abcd"); l.Visits != 3 {
		t.Errorf("Get after 3 visits: Visits = %d, want 3 (did Visit store the update?)", l.Visits)
	}
}

func TestMemoryStoreConcurrentVisits(t *testing.T) {
	s := NewMemoryStore()
	ctx := t.Context()
	if err := s.Create(ctx, Link{Code: "hot1", URL: "https://go.dev"}); err != nil {
		t.Fatal(err)
	}
	const n = 200
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Visit(ctx, "hot1"); err != nil {
				t.Errorf("Visit error = %v", err)
			}
		}()
	}
	wg.Wait()
	if l, _ := s.Get(ctx, "hot1"); l.Visits != n {
		t.Errorf("after %d concurrent visits Visits = %d (lost updates: check your locking)", n, l.Visits)
	}
}

func TestMemoryStoreConcurrentCreateSameCode(t *testing.T) {
	s := NewMemoryStore()
	const n = 50
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins int
	)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Create(t.Context(), Link{Code: "race", URL: "https://go.dev"}); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Errorf("%d concurrent Creates of the same code succeeded %d times, want exactly 1", n, wins)
	}
}

func TestRandomCode(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		code, err := RandomCode()
		if err != nil {
			t.Fatalf("RandomCode error = %v", err)
		}
		if len(code) != 7 || !validCode(code) {
			t.Errorf("RandomCode() = %q, want 7 characters of [A-Za-z0-9]", code)
		}
		if seen[code] {
			t.Errorf("RandomCode() repeated %q within 100 calls", code)
		}
		seen[code] = true
	}
}
