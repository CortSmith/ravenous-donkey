// Package counter implements counters that are safe for concurrent use,
// once with a mutex and once with sync/atomic, plus a per-key counter.
package counter

import (
	"sync"
	"sync/atomic"
)

// Counter is a concurrency-safe counter guarded by a mutex.
// The zero value is ready to use. A Counter must not be copied after first use.
type Counter struct {
	mu sync.Mutex
	n  int64
}

// Inc adds one to the counter.
func (c *Counter) Inc() {
	c.Add(1)
}

// Add adds delta (which may be negative) to the counter.
func (c *Counter) Add(delta int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n += delta
}

// Value returns the current count.
func (c *Counter) Value() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// AtomicCounter is a concurrency-safe counter built on sync/atomic.
// The zero value is ready to use. An AtomicCounter must not be copied after first use.
type AtomicCounter struct {
	n atomic.Int64
}

// Inc adds one to the counter.
func (c *AtomicCounter) Inc() {
	c.n.Add(1)
}

// Add adds delta (which may be negative) to the counter.
func (c *AtomicCounter) Add(delta int64) {
	c.n.Add(delta)
}

// Value returns the current count.
func (c *AtomicCounter) Value() int64 {
	return c.n.Load()
}

// KeyedCounter counts occurrences per string key.
// The zero value is ready to use. A KeyedCounter must not be copied after first use.
type KeyedCounter struct {
	mu sync.Mutex
	m  map[string]int64
}

// Inc adds one to the count for key.
func (c *KeyedCounter) Inc(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]int64)
	}
	c.m[key]++
}

// Get returns the count for key, or 0 if key has never been incremented.
func (c *KeyedCounter) Get(key string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[key] // reading a nil map is fine
}

// Snapshot returns a copy of all counts. The caller owns the returned map;
// changing it does not affect the counter, and later increments do not
// affect it. Snapshot never returns nil.
func (c *KeyedCounter) Snapshot() map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.m))
	for k, v := range c.m {
		out[k] = v
	}
	return out
}
