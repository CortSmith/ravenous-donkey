package shortener

import (
	"context"
	"sync"
)

// MemoryStore is an in-memory Store guarded by a mutex.
type MemoryStore struct {
	mu    sync.Mutex
	links map[string]Link
}

// Compile-time check that MemoryStore satisfies Store.
var _ Store = (*MemoryStore)(nil)

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{links: make(map[string]Link)}
}

// Create implements Store.
func (s *MemoryStore) Create(_ context.Context, l Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.links[l.Code]; ok {
		return ErrExists
	}
	s.links[l.Code] = l
	return nil
}

// Get implements Store.
func (s *MemoryStore) Get(_ context.Context, code string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	return l, nil
}

// Visit implements Store.
func (s *MemoryStore) Visit(_ context.Context, code string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	l.Visits++
	s.links[code] = l
	return l, nil
}
