// Package store implements a concurrency-safe, in-memory key-value store
// with per-key time-to-live (TTL) expiry.
//
// Expiry is enforced twice:
//
//   - lazily: reads treat an expired entry as absent, so a caller can never
//     observe a stale value even if nothing has cleaned it up yet;
//   - actively: a janitor goroutine (see RunJanitor) periodically deletes
//     expired entries so they stop occupying memory.
//
// Milestone 1: implement every function marked TODO. The struct layout,
// New, WithClock and storedLen are given; you may add fields and helpers.
package store

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned by Get when a key is absent or has expired.
var ErrNotFound = errors.New("store: key not found")

// entry is one stored value. A zero expiresAt means "never expires".
type entry struct {
	value     string
	expiresAt time.Time
}

// expired reports whether e is expired at instant now. An entry whose
// expiry equals now is expired: a TTL of 10s means "gone at +10s".
func (e entry) expired(now time.Time) bool {
	// TODO: a zero expiresAt never expires; otherwise expired once now has
	// reached expiresAt. Hint: time.Time has IsZero and Before.
	return false
}

// Option configures a Store.
type Option func(*Store)

// WithClock makes the Store read the current time from now instead of
// time.Now. Tests use it to control expiry without sleeping. now must be
// safe for concurrent use, because the janitor calls it from its own
// goroutine.
func WithClock(now func() time.Time) Option {
	return func(s *Store) { s.now = now }
}

// Store is a concurrency-safe in-memory key-value store. Create one with
// New; the zero value is not usable.
type Store struct {
	now func() time.Time // always call s.now(), never time.Now()

	mu    sync.RWMutex // guards items
	items map[string]entry
}

// New returns an empty Store configured by opts.
func New(opts ...Option) *Store {
	s := &Store{
		now:   time.Now,
		items: make(map[string]entry),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Set stores value under key, replacing any previous value and TTL.
// If ttl > 0 the entry expires ttl after now; otherwise it never expires.
func (s *Store) Set(key, value string, ttl time.Duration) {
	// TODO
}

// Get returns the value stored under key. It returns ErrNotFound if the key
// is absent or expired.
func (s *Store) Get(key string) (string, error) {
	// TODO
	return "", ErrNotFound
}

// Delete removes key and reports whether a live (unexpired) entry was
// removed. An expired entry is removed too, but Delete reports false.
func (s *Store) Delete(key string) bool {
	// TODO
	return false
}

// Keys returns the live keys in ascending order. It never returns expired
// keys. The returned slice is owned by the caller.
func (s *Store) Keys() []string {
	// TODO
	return nil
}

// Len returns the number of live (unexpired) entries.
func (s *Store) Len() int {
	// TODO
	return 0
}

// DeleteExpired removes every expired entry and returns how many it removed.
func (s *Store) DeleteExpired() int {
	// TODO
	return 0
}

// RunJanitor calls DeleteExpired every interval until ctx is canceled, then
// returns. It blocks, so callers usually run it in its own goroutine:
//
//	go s.RunJanitor(ctx, time.Second)
//
// If interval <= 0, RunJanitor returns immediately (the janitor is disabled).
func (s *Store) RunJanitor(ctx context.Context, interval time.Duration) {
	// TODO: create a time.Ticker (and stop it!), then delegate to janitor.
}

// janitor runs one sweep per value received from ticks and returns when ctx
// is canceled. Separating it from RunJanitor lets tests drive sweeps with a
// channel they control instead of a real ticker.
func (s *Store) janitor(ctx context.Context, ticks <-chan time.Time) {
	// TODO: loop forever, selecting on ctx.Done() and ticks.
}

// storedLen returns how many entries are physically held in the map,
// including expired ones the janitor has not removed yet. Tests use it to
// observe the janitor. (Given; do not change.)
func (s *Store) storedLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}
