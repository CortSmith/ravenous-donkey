// Package lru implements a generic least-recently-used cache with optional
// per-entry time-to-live.
//
// See README.md in this directory for the full specification. Run the tests
// with:
//
//	go test ./modules/06-generics/project/
package lru

import (
	"iter"
	"time"
)

// Option configures a Cache. Options are not generic: they set fields that
// don't depend on K or V, so one WithTTL works for every Cache instantiation.
//
// The option plumbing is done for you; the cache itself is your job.
type Option func(*options)

type options struct {
	ttl time.Duration
	now func() time.Time
}

// WithTTL makes entries expire d after they were last Put. A d <= 0 means
// entries never expire (the default).
func WithTTL(d time.Duration) Option {
	return func(o *options) { o.ttl = d }
}

// WithClock replaces time.Now as the cache's source of time. Tests use it to
// control expiry without sleeping.
func WithClock(now func() time.Time) Option {
	return func(o *options) { o.now = now }
}

// Cache is a fixed-capacity LRU cache. Create one with New; the zero value is
// not usable. A Cache is not safe for concurrent use.
type Cache[K comparable, V any] struct {
	// TODO: choose your fields. A map gives O(1) lookup by key; you also
	// need O(1) "move to front" and "remove the oldest". container/list is a
	// good start.
	opts options
}

// New returns an empty cache that holds at most capacity entries. It panics
// if capacity < 1.
func New[K comparable, V any](capacity int, opts ...Option) *Cache[K, V] {
	o := options{now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	// TODO: validate capacity and initialise your fields.
	return &Cache[K, V]{opts: o}
}

// Get returns the value stored for key and marks it most recently used.
// An expired entry is removed and reported as missing.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	var zero V
	// TODO
	return zero, false
}

// Put stores value under key and marks it most recently used. Storing an
// existing key replaces its value and restarts its TTL. If the cache is full,
// the least recently used entry is evicted.
func (c *Cache[K, V]) Put(key K, value V) {
	// TODO
}

// Delete removes key and reports whether it was present (expired or not).
func (c *Cache[K, V]) Delete(key K) bool {
	// TODO
	return false
}

// Len returns the number of stored entries. Expired entries count until they
// are removed by Get, Delete or eviction.
func (c *Cache[K, V]) Len() int {
	// TODO
	return 0
}

// All returns an iterator over the live (unexpired) entries from most to
// least recently used. Iterating does not change recency. The cache must not
// be modified during iteration.
func (c *Cache[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		// TODO
	}
}
