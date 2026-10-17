// Package shortener implements a small URL-shortener HTTP API backed by a
// pluggable Store.
package shortener

import (
	"context"
	"errors"
	"time"
)

// Link is one shortened URL.
type Link struct {
	Code      string    `json:"code"`
	URL       string    `json:"url"`
	Visits    int64     `json:"visits"`
	CreatedAt time.Time `json:"created_at"`
}

// Errors returned by Store implementations. Callers compare with errors.Is.
var (
	ErrNotFound = errors.New("link not found")
	ErrExists   = errors.New("link already exists")
)

// Store persists links. Implementations must be safe for concurrent use.
//
// Every method takes a context even though the in-memory implementation
// ignores it: a database-backed Store needs it for cancellation, and adding
// it later would change every caller.
type Store interface {
	// Create stores l. It returns ErrExists if l.Code is already taken.
	Create(ctx context.Context, l Link) error
	// Get returns the link for code, or ErrNotFound.
	Get(ctx context.Context, code string) (Link, error)
	// Visit atomically increments the visit counter for code and returns the
	// updated link, or ErrNotFound.
	Visit(ctx context.Context, code string) (Link, error)
}
