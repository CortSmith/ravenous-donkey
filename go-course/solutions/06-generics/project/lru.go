// Package lru implements a generic least-recently-used cache with optional
// per-entry time-to-live.
package lru

import (
	"container/list"
	"iter"
	"time"
)

// Option configures a Cache. Options are not generic: they set fields that
// don't depend on K or V, so one WithTTL works for every Cache instantiation.
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

// entry is what each list element holds. The key is stored too, so that
// evicting the back element can also delete it from the map.
type entry[K comparable, V any] struct {
	key     K
	value   V
	expires time.Time // zero when the cache has no TTL
}

// Cache is a fixed-capacity LRU cache. Create one with New; the zero value is
// not usable. A Cache is not safe for concurrent use: even Get modifies it
// (it moves the entry to the front).
type Cache[K comparable, V any] struct {
	capacity int
	opts     options
	ll       *list.List // front = most recently used; elements hold *entry[K, V]
	items    map[K]*list.Element
}

// New returns an empty cache that holds at most capacity entries. It panics
// if capacity < 1, which is a programming error rather than a runtime
// condition (compare make with a negative length).
func New[K comparable, V any](capacity int, opts ...Option) *Cache[K, V] {
	if capacity < 1 {
		panic("lru.New: capacity must be at least 1")
	}
	o := options{now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	return &Cache[K, V]{
		capacity: capacity,
		opts:     o,
		ll:       list.New(),
		items:    make(map[K]*list.Element, capacity),
	}
}

// Get returns the value stored for key and marks it most recently used.
// An expired entry is removed and reported as missing.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	e := el.Value.(*entry[K, V])
	if c.expired(e, c.opts.now()) {
		c.removeElement(el)
		var zero V
		return zero, false
	}
	c.ll.MoveToFront(el)
	return e.value, true
}

// Put stores value under key and marks it most recently used. Storing an
// existing key replaces its value and restarts its TTL. If the cache is full,
// the least recently used entry is evicted.
func (c *Cache[K, V]) Put(key K, value V) {
	var expires time.Time
	if c.opts.ttl > 0 {
		expires = c.opts.now().Add(c.opts.ttl)
	}

	if el, ok := c.items[key]; ok {
		e := el.Value.(*entry[K, V])
		e.value, e.expires = value, expires
		c.ll.MoveToFront(el)
		return
	}

	if c.ll.Len() >= c.capacity {
		c.removeElement(c.ll.Back())
	}
	c.items[key] = c.ll.PushFront(&entry[K, V]{key: key, value: value, expires: expires})
}

// Delete removes key and reports whether it was present (expired or not).
func (c *Cache[K, V]) Delete(key K) bool {
	el, ok := c.items[key]
	if !ok {
		return false
	}
	c.removeElement(el)
	return true
}

// Len returns the number of stored entries. Expired entries count until they
// are removed by Get, Delete or eviction.
func (c *Cache[K, V]) Len() int { return c.ll.Len() }

// All returns an iterator over the live (unexpired) entries from most to
// least recently used. Iterating does not change recency. The cache must not
// be modified during iteration.
func (c *Cache[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		now := c.opts.now()
		for el := c.ll.Front(); el != nil; el = el.Next() {
			e := el.Value.(*entry[K, V])
			if c.expired(e, now) {
				continue
			}
			if !yield(e.key, e.value) {
				return
			}
		}
	}
}

func (c *Cache[K, V]) expired(e *entry[K, V], now time.Time) bool {
	return !e.expires.IsZero() && !now.Before(e.expires)
}

func (c *Cache[K, V]) removeElement(el *list.Element) {
	e := c.ll.Remove(el).(*entry[K, V])
	delete(c.items, e.key)
}
