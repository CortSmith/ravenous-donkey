package inventory

import "errors"

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

// errNotImplemented is a placeholder; delete it when you are done.
var errNotImplemented = errors.New("not implemented")

// MemoryStore is an in-memory Store, safe for concurrent use.
//
// TODO: add the fields you need (a map and something to guard it), choose
// receivers deliberately, and add a compile-time assertion that
// *MemoryStore implements Store.
type MemoryStore struct{}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{} // TODO
}

// Add implements Store.
func (s *MemoryStore) Add(p Product) error { return errNotImplemented }

// Get implements Store.
func (s *MemoryStore) Get(sku string) (Product, error) { return Product{}, errNotImplemented }

// Update implements Store.
func (s *MemoryStore) Update(p Product) error { return errNotImplemented }

// Delete implements Store.
func (s *MemoryStore) Delete(sku string) error { return errNotImplemented }

// List implements Store.
func (s *MemoryStore) List() []Product { return nil }
