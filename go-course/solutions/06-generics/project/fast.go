package lru

import (
	"iter"
	"time"
)

// node is one slot of FastCache's arena. Links are int32 indices into the
// arena rather than pointers: the slice has no internal pointers for the GC
// to chase (unless K or V contain pointers), the nodes sit contiguously in
// memory, and a 32-bit index is half the size of a pointer.
type node[K comparable, V any] struct {
	key        K
	value      V
	expires    int64 // UnixNano; 0 = never. 8 bytes instead of time.Time's 24 (which holds a *Location)
	prev, next int32
}

// FastCache has the same behaviour as Cache but is built for throughput:
//
//   - all nodes live in one preallocated slice, so steady-state Put and Get
//     allocate nothing (container/list allocates an Element per new key, and
//     storing *entry in Element.Value is another allocation);
//   - no interface boxing or type assertions (list.Element.Value is any);
//   - when full, the evicted node's slot is reused for the new entry;
//   - the clock is only consulted when a TTL is configured.
//
// The price is that you now maintain a hand-rolled doubly-linked list with a
// sentinel, which is exactly the kind of code that hides off-by-one bugs.
type FastCache[K comparable, V any] struct {
	nodes []node[K, V] // nodes[0] is the sentinel: next = MRU, prev = LRU
	index map[K]int32
	cap   int
	ttl   int64
	now   func() time.Time
}

// NewFast is the FastCache counterpart of New.
func NewFast[K comparable, V any](capacity int, opts ...Option) *FastCache[K, V] {
	if capacity < 1 {
		panic("lru.NewFast: capacity must be at least 1")
	}
	o := options{now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	c := &FastCache[K, V]{
		nodes: make([]node[K, V], 1, capacity+1),
		index: make(map[K]int32, capacity),
		cap:   capacity,
		ttl:   max(int64(o.ttl), 0),
		now:   o.now,
	}
	return c // sentinel's prev/next are 0: an empty circular list
}

func (c *FastCache[K, V]) unlink(i int32) {
	n := &c.nodes[i]
	c.nodes[n.prev].next = n.next
	c.nodes[n.next].prev = n.prev
}

func (c *FastCache[K, V]) pushFront(i int32) {
	head := c.nodes[0].next
	c.nodes[i].prev, c.nodes[i].next = 0, head
	c.nodes[head].prev = i
	c.nodes[0].next = i
}

func (c *FastCache[K, V]) nowNano() int64 {
	if c.ttl == 0 {
		return 0
	}
	return c.now().UnixNano()
}

func (c *FastCache[K, V]) isExpired(n *node[K, V], now int64) bool {
	return n.expires != 0 && now >= n.expires
}

// Get behaves like Cache.Get.
func (c *FastCache[K, V]) Get(key K) (V, bool) {
	i, ok := c.index[key]
	if !ok {
		var zero V
		return zero, false
	}
	n := &c.nodes[i]
	if c.isExpired(n, c.nowNano()) {
		c.remove(i)
		var zero V
		return zero, false
	}
	if c.nodes[0].next != i {
		c.unlink(i)
		c.pushFront(i)
	}
	return n.value, true
}

// Put behaves like Cache.Put.
func (c *FastCache[K, V]) Put(key K, value V) {
	var expires int64
	if c.ttl > 0 {
		expires = c.now().UnixNano() + c.ttl
	}
	if i, ok := c.index[key]; ok {
		c.nodes[i].value, c.nodes[i].expires = value, expires
		c.unlink(i)
		c.pushFront(i)
		return
	}

	var i int32
	if len(c.index) >= c.cap {
		i = c.nodes[0].prev // reuse the LRU slot
		delete(c.index, c.nodes[i].key)
		c.unlink(i)
	} else {
		// Grow the arena. After a Delete there may be a hole; remove() keeps
		// the arena dense by moving the last node into it, so the next free
		// slot is always len(c.nodes).
		c.nodes = append(c.nodes, node[K, V]{})
		i = int32(len(c.nodes) - 1)
	}
	c.nodes[i].key, c.nodes[i].value, c.nodes[i].expires = key, value, expires
	c.index[key] = i
	c.pushFront(i)
}

// remove unlinks slot i and keeps the arena dense by moving the last node
// into the hole.
func (c *FastCache[K, V]) remove(i int32) {
	c.unlink(i)
	delete(c.index, c.nodes[i].key)
	last := int32(len(c.nodes) - 1)
	if i != last {
		c.nodes[i] = c.nodes[last]
		n := &c.nodes[i]
		c.nodes[n.prev].next = i
		c.nodes[n.next].prev = i
		c.index[n.key] = i
	}
	c.nodes[last] = node[K, V]{} // drop references held by K/V
	c.nodes = c.nodes[:last]
}

// Delete behaves like Cache.Delete.
func (c *FastCache[K, V]) Delete(key K) bool {
	i, ok := c.index[key]
	if !ok {
		return false
	}
	c.remove(i)
	return true
}

// Len behaves like Cache.Len.
func (c *FastCache[K, V]) Len() int { return len(c.index) }

// All behaves like Cache.All.
func (c *FastCache[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		now := c.nowNano()
		for i := c.nodes[0].next; i != 0; i = c.nodes[i].next {
			n := &c.nodes[i]
			if c.isExpired(n, now) {
				continue
			}
			if !yield(n.key, n.value) {
				return
			}
		}
	}
}
