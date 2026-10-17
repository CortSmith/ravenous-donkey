package store

import (
	"slices"
	"sync"
	"time"
)

// shardCount is the number of independent shards in a ShardedStore. A power
// of two lets us pick a shard with a mask instead of a modulo.
const shardCount = 32

// shard is one independently locked slice of the key space. The padding
// keeps neighbouring shards' mutexes on different cache lines, so two cores
// locking adjacent shards do not invalidate each other's cache line (false
// sharing).
type shard struct {
	mu    sync.RWMutex
	items map[string]entry
	_     [128 - 32]byte // RWMutex (24) + map pointer (8) = 32; pad to two cache lines
}

// ShardedStore is a performance-oriented variant of Store. It splits the key
// space across shardCount maps, each with its own lock, so operations on
// different keys rarely contend. Get, Set and Delete have the same semantics
// as Store's; only Keys, Len and DeleteExpired have to visit every shard.
type ShardedStore struct {
	now    func() time.Time
	shards [shardCount]shard
}

// NewSharded returns an empty ShardedStore.
func NewSharded(now func() time.Time) *ShardedStore {
	if now == nil {
		now = time.Now
	}
	s := &ShardedStore{now: now}
	for i := range s.shards {
		s.shards[i].items = make(map[string]entry)
	}
	return s
}

// shardFor hashes key with inline FNV-1a. hash/fnv would need a []byte
// conversion and an interface call; this loop allocates nothing.
func (s *ShardedStore) shardFor(key string) *shard {
	h := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return &s.shards[h&(shardCount-1)]
}

// Set behaves like Store.Set.
func (s *ShardedStore) Set(key, value string, ttl time.Duration) {
	e := entry{value: value}
	if ttl > 0 {
		e.expiresAt = s.now().Add(ttl)
	}
	sh := s.shardFor(key)
	sh.mu.Lock()
	sh.items[key] = e
	sh.mu.Unlock() // no defer: the critical section cannot panic
}

// Get behaves like Store.Get.
func (s *ShardedStore) Get(key string) (string, error) {
	now := s.now()
	sh := s.shardFor(key)
	sh.mu.RLock()
	e, ok := sh.items[key]
	sh.mu.RUnlock()
	if !ok || e.expired(now) {
		return "", ErrNotFound
	}
	return e.value, nil
}

// Delete behaves like Store.Delete.
func (s *ShardedStore) Delete(key string) bool {
	now := s.now()
	sh := s.shardFor(key)
	sh.mu.Lock()
	e, ok := sh.items[key]
	delete(sh.items, key)
	sh.mu.Unlock()
	return ok && !e.expired(now)
}

// Keys behaves like Store.Keys. It locks one shard at a time, so the result
// is not an atomic snapshot of the whole store; that is the price of
// sharding.
func (s *ShardedStore) Keys() []string {
	now := s.now()
	var keys []string
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.RLock()
		for k, e := range sh.items {
			if !e.expired(now) {
				keys = append(keys, k)
			}
		}
		sh.mu.RUnlock()
	}
	slices.Sort(keys)
	return keys
}

// DeleteExpired behaves like Store.DeleteExpired, but holds each shard's
// lock only while sweeping that shard, so readers of other shards are
// never blocked by the sweep.
func (s *ShardedStore) DeleteExpired() int {
	now := s.now()
	n := 0
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.Lock()
		for k, e := range sh.items {
			if e.expired(now) {
				delete(sh.items, k)
				n++
			}
		}
		sh.mu.Unlock()
	}
	return n
}
