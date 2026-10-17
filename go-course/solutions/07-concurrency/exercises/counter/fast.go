package counter

import (
	"hash/maphash"
	"sync"
)

// shardCount is a power of two so the shard index is a cheap mask.
const shardCount = 32

// ShardedKeyedCounter is a KeyedCounter split into independently locked
// shards. Goroutines touching different keys usually take different locks,
// so contention drops roughly by the shard count. The zero value is NOT
// ready to use; call NewShardedKeyedCounter.
type ShardedKeyedCounter struct {
	seed   maphash.Seed
	shards [shardCount]struct {
		mu sync.Mutex
		m  map[string]int64
		_  [48]byte // pad to a 64-byte cache line to avoid false sharing between shard locks
	}
}

// NewShardedKeyedCounter returns an empty ShardedKeyedCounter.
func NewShardedKeyedCounter() *ShardedKeyedCounter {
	c := &ShardedKeyedCounter{seed: maphash.MakeSeed()}
	for i := range c.shards {
		c.shards[i].m = make(map[string]int64)
	}
	return c
}

func (c *ShardedKeyedCounter) shard(key string) int {
	return int(maphash.String(c.seed, key) & (shardCount - 1))
}

// Inc adds one to the count for key.
func (c *ShardedKeyedCounter) Inc(key string) {
	s := &c.shards[c.shard(key)]
	s.mu.Lock()
	s.m[key]++
	s.mu.Unlock()
}

// Get returns the count for key.
func (c *ShardedKeyedCounter) Get(key string) int64 {
	s := &c.shards[c.shard(key)]
	s.mu.Lock()
	n := s.m[key]
	s.mu.Unlock()
	return n
}

// Snapshot returns a copy of all counts. It locks one shard at a time, so
// the result is not a single atomic point-in-time view across shards.
func (c *ShardedKeyedCounter) Snapshot() map[string]int64 {
	out := make(map[string]int64)
	for i := range c.shards {
		s := &c.shards[i]
		s.mu.Lock()
		for k, v := range s.m {
			out[k] = v
		}
		s.mu.Unlock()
	}
	return out
}
