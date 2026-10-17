package inventory

import (
	"cmp"
	"fmt"
	"slices"
	"sync"
)

// Store persists products by SKU. Implementations must be safe for
// concurrent use and must return copies, so callers can never mutate stored
// state behind the store's back.
type Store interface {
	// Add inserts p. It returns an error wrapping ErrDuplicate if the SKU
	// already exists.
	Add(p Product) error
	// Get returns the product with the given SKU, or an error wrapping
	// ErrNotFound.
	Get(sku string) (Product, error)
	// Update replaces an existing product (matched by SKU), or returns an
	// error wrapping ErrNotFound.
	Update(p Product) error
	// Delete removes the product, or returns an error wrapping ErrNotFound.
	Delete(sku string) error
	// List returns all products sorted by SKU. The slice is newly allocated.
	List() []Product
}

// MemoryStore is an in-memory Store. Use NewMemoryStore to create one; it
// must not be copied after first use because it contains a mutex.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]Product
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string]Product)}
}

// Add implements Store.
func (s *MemoryStore) Add(p Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[p.SKU]; ok {
		return fmt.Errorf("add %s: %w", p.SKU, ErrDuplicate)
	}
	s.items[p.SKU] = p
	return nil
}

// Get implements Store.
func (s *MemoryStore) Get(sku string) (Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.items[sku]
	if !ok {
		return Product{}, fmt.Errorf("get %s: %w", sku, ErrNotFound)
	}
	return p, nil
}

// Update implements Store.
func (s *MemoryStore) Update(p Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[p.SKU]; !ok {
		return fmt.Errorf("update %s: %w", p.SKU, ErrNotFound)
	}
	s.items[p.SKU] = p
	return nil
}

// Delete implements Store.
func (s *MemoryStore) Delete(sku string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[sku]; !ok {
		return fmt.Errorf("delete %s: %w", sku, ErrNotFound)
	}
	delete(s.items, sku)
	return nil
}

// List implements Store.
func (s *MemoryStore) List() []Product {
	s.mu.RLock()
	out := make([]Product, 0, len(s.items))
	for _, p := range s.items {
		out = append(out, p)
	}
	s.mu.RUnlock()
	slices.SortFunc(out, func(a, b Product) int { return cmp.Compare(a.SKU, b.SKU) })
	return out
}
