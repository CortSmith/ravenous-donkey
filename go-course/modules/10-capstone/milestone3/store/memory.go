// Package store is a persistent key-value store: the in-memory, TTL-aware
// Store from milestone 1 (memory.go, given) made durable by an append-only
// log (log.go) and tied together by DB (db.go).
//
// memory.go is the milestone 1 reference solution plus two methods that
// persistence needs, SetExpiresAt and Items. If you have not finished
// milestone 1 yet, do that first.
package store

import (
	"context"
	"errors"
	"slices"
	"strings"
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
	return !e.expiresAt.IsZero() && !now.Before(e.expiresAt)
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
	now func() time.Time

	mu    sync.RWMutex
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
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = s.now().Add(ttl)
	}
	s.SetExpiresAt(key, value, expiresAt)
}

// SetExpiresAt stores value under key with an absolute expiry time. A zero
// expiresAt means the entry never expires. An expiresAt in the past stores
// an already-expired entry, which reads treat as absent.
//
// Persistence needs this form: a TTL is relative to "now", but the log must
// record the instant the key dies so a restart does not extend its life.
func (s *Store) SetExpiresAt(key, value string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = entry{value: value, expiresAt: expiresAt}
}

// Get returns the value stored under key. It returns ErrNotFound if the key
// is absent or expired.
func (s *Store) Get(key string) (string, error) {
	now := s.now()

	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.items[key]
	if !ok || e.expired(now) {
		return "", ErrNotFound
	}
	return e.value, nil
}

// Delete removes key and reports whether a live (unexpired) entry was
// removed.
func (s *Store) Delete(key string) bool {
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok {
		return false
	}
	delete(s.items, key)
	return !e.expired(now)
}

// Keys returns the live keys in ascending order. It never returns expired
// keys. The returned slice is owned by the caller.
func (s *Store) Keys() []string {
	now := s.now()

	s.mu.RLock()
	keys := make([]string, 0, len(s.items))
	for k, e := range s.items {
		if !e.expired(now) {
			keys = append(keys, k)
		}
	}
	s.mu.RUnlock()

	slices.Sort(keys) // sort outside the lock: it only touches our copy
	return keys
}

// Item is a snapshot of one live entry, as returned by Items.
type Item struct {
	Key       string
	Value     string
	ExpiresAt time.Time // zero means never expires
}

// Items returns a snapshot of every live entry, sorted by key.
func (s *Store) Items() []Item {
	now := s.now()

	s.mu.RLock()
	items := make([]Item, 0, len(s.items))
	for k, e := range s.items {
		if !e.expired(now) {
			items = append(items, Item{Key: k, Value: e.value, ExpiresAt: e.expiresAt})
		}
	}
	s.mu.RUnlock()

	slices.SortFunc(items, func(a, b Item) int { return strings.Compare(a.Key, b.Key) })
	return items
}

// Len returns the number of live (unexpired) entries.
func (s *Store) Len() int {
	now := s.now()

	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, e := range s.items {
		if !e.expired(now) {
			n++
		}
	}
	return n
}

// DeleteExpired removes every expired entry and returns how many it removed.
func (s *Store) DeleteExpired() int {
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for k, e := range s.items {
		if e.expired(now) {
			delete(s.items, k) // deleting during range is allowed by the spec
			n++
		}
	}
	return n
}

// RunJanitor calls DeleteExpired every interval until ctx is canceled, then
// returns. It blocks, so callers usually run it in its own goroutine:
//
//	go s.RunJanitor(ctx, time.Second)
//
// If interval <= 0, RunJanitor returns immediately (the janitor is disabled).
func (s *Store) RunJanitor(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	s.janitor(ctx, t.C)
}

// janitor runs one sweep per value received from ticks and returns when ctx
// is canceled. Separating it from RunJanitor lets tests drive sweeps with a
// channel they control instead of a real ticker.
func (s *Store) janitor(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			s.DeleteExpired()
		}
	}
}
