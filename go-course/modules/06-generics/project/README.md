# Project: Generic LRU Cache

**Time budget:** 45 to 60 minutes.

You will build `Cache[K comparable, V any]`, a fixed-capacity cache that evicts the
**least recently used** entry when it is full. Entries can optionally expire after a
time-to-live (TTL), and the cache exposes its contents as an `iter.Seq2[K, V]` in
recency order.

LRU caches are a classic interview question for good reason: they need two data
structures working together, and the invariants between them are easy to break. They
are also a natural fit for generics, because the algorithm does not care what the keys
and values are.

## Getting started

Everything lives in `modules/06-generics/project/`:

| File | What it is |
|---|---|
| `lru.go` | Starter code. The option plumbing (`Option`, `WithTTL`, `WithClock`) is done. Fill in the `Cache` fields and every method marked `TODO`. |
| `lru_test.go` | The acceptance tests. Don't edit them. |

Run the tests with:

```sh
go test ./modules/06-generics/project/
```

Run a single test while you work:

```sh
go test -run 'TestEvictsLeastRecentlyUsed' -v ./modules/06-generics/project/
```

## Specification

### Construction

```go
func New[K comparable, V any](capacity int, opts ...Option) *Cache[K, V]
```

- `capacity` is the maximum number of entries. `New` **panics** if `capacity < 1`. A
  non-positive capacity is a bug in the caller, not a condition they can recover from
  at run time. `make([]int, -1)` panics for the same reason.
- Options:
  - `WithTTL(d)`: each entry expires `d` after it was last **Put**. `d <= 0` means no
    expiry (the default).
  - `WithClock(now)`: the cache uses `now()` instead of `time.Now()`. The tests pass a
    fake clock so they never sleep.
- Call sites must spell out the type arguments, e.g. `lru.New[string, int](100)`,
  because `K` and `V` appear in no parameter and so can't be inferred. That's normal
  for constructors of generic containers.

### Operations

| Method | Behaviour |
|---|---|
| `Get(key) (V, bool)` | Returns the value and `true`, and marks the entry most recently used. A missing key returns the zero `V` and `false`. If the entry has expired, `Get` **removes it** and reports it as missing. `Get` does **not** extend the TTL. |
| `Put(key, value)` | Inserts or replaces. The entry becomes most recently used and its TTL restarts. Replacing an existing key never evicts anything. Inserting a new key into a full cache first evicts the least recently used entry. |
| `Delete(key) bool` | Removes the entry, expired or not. Reports whether it was present. |
| `Len() int` | Number of stored entries. Expired entries **still count** until something removes them (`Get`, `Delete`, or eviction). Purging them eagerly would make `Len` O(n). |
| `All() iter.Seq2[K, V]` | Yields live (unexpired) entries from **most to least** recently used. It must not change recency, must stop as soon as `yield` returns false, and the returned sequence must be reusable. |

An entry is expired when `now >= putTime + ttl`, so at exactly `ttl` it is already gone.

### Complexity

`Get`, `Put` and `Delete` must be **O(1)**. `All` is O(n). A slice that you shift or
search on every access doesn't meet this requirement, even though it passes the tests.

### Concurrency

`Cache` is **not** safe for concurrent use, and you shouldn't add a mutex in this
project. Concurrency is the topic of module 07. Note that even `Get` *writes* (it moves
the entry to the front), so a `sync.RWMutex` with `RLock` in `Get` would be a data race.

## Hints

<details>
<summary>Hint 1: which data structures?</summary>

Use a `map[K]*list.Element` for O(1) lookup, plus a `container/list` doubly linked
list ordered by recency (front = most recent). `MoveToFront`, `PushFront`, `Back` and
`Remove` are all O(1).

</details>

<details>
<summary>Hint 2: what goes in the list element?</summary>

`list.Element.Value` has type `any`. Store a pointer to your own generic struct:

```go
type entry[K comparable, V any] struct {
	key     K
	value   V
	expires time.Time
}
```

You need the **key** in the entry. When you evict `ll.Back()`, you have to delete that
key from the map, and the list element is all you have at that point. Read it back with
a type assertion: `el.Value.(*entry[K, V])`.

</details>

<details>
<summary>Hint 3: TTL</summary>

Compute `expires := now().Add(ttl)` in `Put` when `ttl > 0`, and leave it as the zero
`time.Time` otherwise. An entry is expired if `!expires.IsZero() && !now.Before(expires)`.

</details>

## Acceptance criteria

- [ ] `go test ./modules/06-generics/project/` passes.
- [ ] `go vet ./modules/06-generics/project/` is clean and the code is `gofmt`ed.
- [ ] `Get`/`Put`/`Delete` are O(1).
- [ ] No exported identifiers beyond those in the starter.
- [ ] Every exported identifier has a doc comment that starts with its name.
- [ ] Every nontrivial piece of logic (eviction, expiry check, removal) exists exactly
      once. For example, a `removeElement` helper that removes from both the list and
      the map.

## Stretch goals

Do these only once the tests pass. Add your own tests for each.

1. **`Peek(key) (V, bool)`**: like `Get` but doesn't change recency. Should it remove
   expired entries? Pick an answer and document it.
2. **Eviction callback**: `WithOnEvict` can't be a plain `Option`, because the callback
   needs `K` and `V` (`func(K, V)`). Find a design that works. One approach is a generic
   option type, `type CacheOption[K comparable, V any] func(*Cache[K, V])`. What does
   that do to call sites?
3. **Prefer expired victims**: when the cache is full, evict an expired entry (if there
   is one) before the true LRU entry. What does that do to `Put`'s complexity? Is there
   a cheap approximation?
4. **Zero-allocation version**: replace `container/list` with a slice of nodes linked by
   `int32` indices, and reuse the evicted node's slot. Write a benchmark with `b.Loop()`
   and compare `-benchmem` output with the `container/list` version.
5. **Thread safety** (preview of module 07): add a `sync.Mutex`, then run the tests
   with `-race` while several goroutines `Get` and `Put`.

## Submitting

When the tests pass, ask your instructor (Claude) to review `modules/06-generics/`.
