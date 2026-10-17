package lookup

import (
	"errors"
	"testing"
)

func TestFastMapStoreMatchesMapStore(t *testing.T) {
	fast := FastMapStore(users)
	for _, id := range []int{1, 2, 3, 42, -7} {
		wantU, wantErr := users.Get(id)
		gotU, gotErr := fast.Get(id)
		if gotU != wantU {
			t.Errorf("FastMapStore.Get(%d) user = %+v, want %+v", id, gotU, wantU)
		}
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("FastMapStore.Get(%d) err = %v, want %v", id, gotErr, wantErr)
		}
		if gotErr != nil {
			if gotErr.Error() != wantErr.Error() {
				t.Errorf("FastMapStore.Get(%d) err = %q, want %q", id, gotErr, wantErr)
			}
			if !errors.Is(gotErr, ErrNotFound) {
				t.Errorf("errors.Is(FastMapStore.Get(%d), ErrNotFound) = false", id)
			}
		}
	}
	s := &Service{Users: fast}
	if got, err := s.Greet(404); err != nil || got != "Hello, guest!" {
		t.Errorf("Greet(404) with FastMapStore = %q, %v; want guest", got, err)
	}
}

func BenchmarkGreetMissMapStore(b *testing.B) {
	s := &Service{Users: users}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.Greet(404); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGreetMissFastMapStore(b *testing.B) {
	s := &Service{Users: FastMapStore(users)}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.Greet(404); err != nil {
			b.Fatal(err)
		}
	}
}

var sinkErr error

func BenchmarkGetMissMapStore(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, sinkErr = users.Get(404)
	}
}

func BenchmarkGetMissFastMapStore(b *testing.B) {
	fast := FastMapStore(users)
	b.ReportAllocs()
	for b.Loop() {
		_, sinkErr = fast.Get(404)
	}
}
