// Package lookup demonstrates sentinel errors travelling through layers:
// a storage layer reports "not found", a service layer adds context, and a
// top-level caller decides what "not found" means for the user.
package lookup

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is returned (wrapped) when a user does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalidID is returned (wrapped) for IDs that can never exist (<= 0).
var ErrInvalidID = errors.New("invalid id")

// User is a stored user record.
type User struct {
	ID   int
	Name string
}

// Getter is the storage dependency of Service. It is defined here, by the
// consumer, so tests can substitute any implementation.
type Getter interface {
	Get(id int) (User, error)
}

// MapStore is an in-memory Getter.
type MapStore map[int]User

// Get returns the user with the given ID. A missing user yields an error
// that wraps ErrNotFound and mentions the ID: "user 7: not found".
func (m MapStore) Get(id int) (User, error) {
	u, ok := m[id]
	if !ok {
		return User{}, fmt.Errorf("user %d: %w", id, ErrNotFound)
	}
	return u, nil
}

// Service implements user-facing operations on top of a Getter.
type Service struct {
	Users Getter
}

// DisplayName returns the user's trimmed name, or "user #ID" when the stored
// name is blank.
//
// Errors:
//   - id <= 0: "user ID: invalid id" (wraps ErrInvalidID); the store is not called.
//   - store failure: "user store: <store error>" (wraps the store's error).
func (s *Service) DisplayName(id int) (string, error) {
	if id <= 0 {
		return "", fmt.Errorf("user %d: %w", id, ErrInvalidID)
	}
	u, err := s.Users.Get(id)
	if err != nil {
		return "", fmt.Errorf("user store: %w", err)
	}
	if name := strings.TrimSpace(u.Name); name != "" {
		return name, nil
	}
	return fmt.Sprintf("user #%d", id), nil
}

// Greet returns "Hello, NAME!". A user that does not exist is greeted as
// "Hello, guest!" with a nil error: for this caller "not found" is an
// expected outcome, not a failure. Every other error is returned wrapped as
// "greet: <error>".
func (s *Service) Greet(id int) (string, error) {
	name, err := s.DisplayName(id)
	switch {
	case errors.Is(err, ErrNotFound):
		return "Hello, guest!", nil
	case err != nil:
		return "", fmt.Errorf("greet: %w", err)
	}
	return "Hello, " + name + "!", nil
}
