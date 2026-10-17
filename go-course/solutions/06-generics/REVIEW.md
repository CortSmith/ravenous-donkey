# Review: Generics & Iterators

This document provides reference solutions and grading rubrics for the 06-generics module. Every exercise and the project are covered with readable and optimized implementations, trade-off analysis, and specific review checklists to help instructors grade consistently.

---

## Exercise 1: `funcs` — Higher-order functions

### Readable solution

The readable versions of these functions use the straightforward idiom: iterate the input, transform/filter, and collect results. `Map` pre-allocates the result slice to the input length. `Filter` appends conditionally to a fresh slice (not the caller's backing array) to avoid modifying or corrupting the input. `Reduce` threads an accumulator through the elements and returns it unchanged when the slice is empty. `GroupBy` builds a map keyed by the `key` function's return value, ensuring it always returns a non-nil map.

The constraint pattern `S ~[]E` preserves the caller's slice type in the result, so `Filter` applied to a `type Names []string` returns `Names`, not plain `[]string`. This is crucial for maintaining type identity.

### Optimized solution

Three optimized variants exist beyond the readable `Map` and `Filter`:

- **MapFast:** Instead of `append` into a slice made with `make([]R, 0, len(s))`, make the slice with its full length and assign by index. The readable version never regrows either (its capacity is already exact), so the only saving is `append`'s per-call capacity check and length update. **Measured on Go 1.24.2 linux/amd64 (`-count=5`, medians):** Map: ~48,500 ns/op (81,920 B, 1 alloc); MapFast: ~45,400 ns/op (81,920 B, 1 alloc). About 5–10% faster, which is within run-to-run noise on this machine. Same memory. Lesson: once capacity is right, `append` is nearly free.

- **FilterFast:** Pre-allocate the worst-case capacity (len(s)) up front, so it does exactly one allocation no matter how many elements survive. The cost is memory: if `keep` rejects most elements, you pin a mostly-empty backing array long-term. Callers can recover space with `slices.Clip` if needed. **Measured (medians):** Filter (readable): ~76,700 ns/op (128,248 B, 16 allocs); FilterFast (prealloc): ~60,700 ns/op (81,920 B, 1 alloc). About 20–35% faster depending on the run, single allocation instead of 16.

- **FilterInPlace:** Reslice the caller's backing array to 0 and append kept elements into it. This DESTROYS the input slice—only correct when the caller owns it entirely and won't use it again. Zero allocations. This is what `slices.DeleteFunc` does (with the predicate inverted). **Measured:** FilterInPlace: ~23,400 ns/op (0 B, 0 allocs). This is about 3x faster than readable Filter, but correctness depends on ownership and contract clarity. The speedup comes from both eliminating allocations and reducing work (append into pre-owned capacity).

### Trade-off

- **Readable vs MapFast:** The measured difference (~5–10%) is close to noise, and the indexed loop is about as clear as `append`. Either is fine; don't flag one or the other in review.
- **Readable Filter vs FilterFast:** Readable Filter's 16 allocations come from repeated slice growth. FilterFast cuts the time by roughly a quarter and the allocations to one, paying with worst-case memory. Choose FilterFast when allocations are profiled as a bottleneck.
- **FilterInPlace vs others:** Only use FilterInPlace when you own the input and never intend to read it again. The zero-allocation property is powerful in tight loops (e.g., a cleanup phase), but the precondition is strict. Document it prominently; it's easy to misuse.

### Review checklist

- Does `Map` pre-allocate with exactly `len(s)` capacity? (Not larger, not smaller.)
- Does `Filter` build a fresh slice with `var out S` (appending to a nil slice) instead of `s[:0]`? The latter overwrites `s` itself.
- Does `Reduce` return the accumulator itself when the slice is empty? (Not a zero `R`, but the `init` value passed in.)
- Does `GroupBy` avoid a misleading size hint? The number of groups is unknown, so `make(map[K]S, len(s))` usually over-allocates; plain `make(map[K]S)` is the right default.
- Does `GroupBy` return a non-nil map even when `s` is empty? (Important for JSON marshaling; `nil` and `{}` marshal differently.)
- All four functions: do they preserve the input `s` unchanged and never mutate its backing array?
- Why is `K comparable` in `GroupBy` but not required on `E` in `Map`? (Because map keys must be comparable, but elements can be anything.)
- Are all doc comments present and start with the function name?

---

## Exercise 2: `set` — A generic hash set

### Readable solution

`Set[T comparable]` wraps `map[T]struct{}`, the idiomatic Go set. The zero value `Set` is usable: reading from a nil map is safe (returns the zero value), only writes panic, so `Add` initializes `s.m` on first write while `Remove` and `Contains` just read. `Contains` and `Len` additionally accept a nil `*Set` (they check `s == nil`), so a nil set behaves like an empty one for reads. `Add` on a nil `*Set` still panics, which is acceptable: there is nowhere to store the element.

`All()` returns `iter.Seq[T]`, allowing callers to range over the set lazily. Set operations (`Union`, `Intersect`, `Difference`) use the iterator and build new sets by iterating and conditionally adding. This is obviously correct and easy to verify.

`Sorted` is a standalone function, not a method, because it needs the constraint `T cmp.Ordered` while `Set` only requires `T comparable`. A method cannot tighten the receiver's type constraint, so the function form is necessary.

The readable implementation prioritizes clarity: direct map operations, no optimizations, obviously correct logic.

### Optimized solution

`IntersectFast` uses an asymmetry optimization: it walks the **smaller** of the two sets and probes the **larger**, doing O(min(len(s), len(o))) map lookups instead of O(len(s)). It also pre-allocates the result map to the smaller set's size, avoiding incremental map growth during insertion.

**Measured on Go 1.24.2 linux/amd64:** Two sets of 100,000 and 100 integers respectively (Readable version iterates all 100k; Fast version iterates the 100).
- Intersect (readable): ~2,470,000 ns/op (4,656 B, 12 allocs)
- IntersectFast (fast): ~4,100 ns/op (2,400 B, 5 allocs)
- **Speedup: ~600x faster** (the exact ratio varies between runs; the point is orders of magnitude). This dramatic improvement happens because the fast version avoids iterating 100k elements; instead it only probes 100 lookups. This illustrates how algorithmic choice (not just implementation detail) drives performance.

The code is less obvious: the asymmetry swap and direct `range` over `s.m` (avoiding the iterator closure) require careful reading, but the test coverage is thorough.

### Trade-off

- **Readable vs IntersectFast:** Readable is fine unless you're intersecting huge sets repeatedly in hot code. The iterator overhead is real but much smaller than Intersect's algorithmic penalty. The clarity of "I range over s and check if each is in o" beats the asymmetry optimization in most contexts. Use IntersectFast only if profiling shows intersection is a bottleneck.

### Review checklist

- Does `Add` initialize `s.m` if it's nil, or rely on an `ensure()` helper?
- Do the read-only methods (`Contains`, `Len`) handle a nil `*Set` safely, returning false/0 instead of panicking?
- Does the iterator in `All()` check `yield`'s return value and return immediately when it's false? Critical for early break: `if !yield(v) { return }`.
- Does `Sorted` use a top-level generic function instead of a method? (Go 1.24 doesn't support generic methods; a receiver's constraints are fixed.)
- Do set operations return non-nil maps for consistency? `Union(nil, nil)` should be `{}`, not `nil`. `len(Union(nil, nil))` should be `0`, not a panic.
- For `IntersectFast`: does the asymmetry correctly swap if s is larger than o?
- Are the comment explanations clear about why Intersect was not optimized in the readable version?

---

## Exercise 3: `stackqueue` — Generic LIFO and FIFO containers

### Readable solution

`Stack[T]` is a slice where `Push` appends and `Pop` removes the last element, returning it and a bool for success. `Queue[T]` is a slice where `Push` appends and `Pop` removes the first element by reslicing (`items[1:]`).

Both return the zero value of `T` when empty, avoiding panics and simplifying the API. Critically, after popping, the code explicitly zeroes the vacated slot to allow the GC to reclaim anything the old value points to. For Stack: `s.items[last] = zero` after `Pop`. For Queue: `q.items[0] = zero` before `q.items = q.items[1:]`. The slot is now outside the slice, but it is still part of the backing array until the next regrow, so without zeroing it would keep the old value alive. A student who forgets this in either type has a memory leak whenever `T` holds pointers and the container is long-lived.

Both are simple and obviously correct. The zero-value readiness makes them usable as struct fields without initialization.

### Optimized solution

`RingQueue[T]` uses a **circular buffer** backed by a power-of-two-sized slice. Head and tail pointers (represented as a head index and a count `n`) never grow. `Push` reallocates only when the buffer is genuinely full. `Pop` never shrinks. In steady state (equal push/pop), RingQueue allocates nothing after initial growth; `Queue` by contrast keeps sliding its window forward, eventually exhausting capacity and forcing `append` to copy the entire slice.

The ring logic uses bitwise `&` for modulo because the buffer size is a power of two. When the buffer is full, grow() unrolls the circular buffer into a new, contiguous block.

**Measured on Go 1.24.2 linux/amd64:** 1000-element steady-state churn (one push and one pop per iteration):
- Queue (slice): ~12.3 ns/op (22 B/op, 0 allocs — the allocation count rounds down to 0 because it is less than one per op)
- RingQueue (ring): ~2.9 ns/op (0 B/op, 0 allocs)
- **Speedup: ~4x faster.** Reslicing with `items[1:]` never allocates. But it moves the slice's start forward through the backing array, and the slots in front are lost. When `append` reaches the end of the capacity, it allocates a new array and copies the live elements. That periodic copy, averaged over every op, is the 22 B/op. RingQueue reuses its slots, so it never reallocates in steady state.

The trade-off: the ring logic is harder to verify; it's easy to get off-by-one bugs wrong. The sentinel pattern and index wrapping require careful review.

### Trade-off

- **Readable vs RingQueue:** The slice-based Queue is dead simple and fine for most uses. RingQueue saves allocations in a tight loop but its ring logic is harder to verify and the speedup is architectural, not a small constant factor. Use RingQueue if profiling shows the queue is allocating in a hot path. For interactive applications or batch processing, Queue is sufficient.

### Review checklist

- Do both `Pop()` and `Peek()` return the zero value and false when the container is empty? (Not panicking.)
- Does `Pop` zero the vacated slot to avoid pinning values the GC should reclaim? Critical for Stack; important for Queue if elements point to large allocations.
- Is the zero value of both types usable (empty and ready for `Push` without initialization)?
- For RingQueue (fast.go):
  - Does `grow()` correctly unroll the circular buffer when the head is not 0? (Copy from head to end, then from start to head, into the new buffer.)
  - Is the index wrapping correct? `(q.head + q.n) & (len(q.buf) - 1)` for the insert position when pushing.
  - Does `Pop` update both the vacated slot's value and the head index?
- Are there any off-by-one bugs in the circular-list manipulation? This is the most error-prone part.

---

## Exercise 4: `numeric` — Arithmetic over type constraints

### Readable solution

Define three constraints:

- **Integer** (all signed and unsigned integer types and their named variants): `~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ...`
- **Float** (float32, float64 and their named variants): `~float32 | ~float64`
- **Number** (the union): `Integer | Float`

Each uses the `~` prefix because a named type like `type Celsius float64` must satisfy `~float64`.

`Sum` accumulates a `T` in a local variable and returns it unchanged when the slice is empty. `SumBy` takes a slice of structs and a function to extract a `Number` from each, useful for summing a field across a struct slice without a separate step.

`Mean` converts to `float64` to avoid integer overflow in the accumulator. It returns `(float64, bool)` because the mean of an empty slice is undefined (`0/0.0` would be NaN). The bool signals success/failure.

`Clamp` uses `cmp.Ordered` (integers, floats, strings) and returns `min(max(v, lo), hi)` with the Go 1.21 builtins. It panics if `cmp.Less(hi, lo)`. `cmp.Less` orders NaN before every other value, whereas `hi < lo` is always false when either is NaN. Note that the builtin `min`/`max` propagate NaN, so `Clamp(NaN, 0, 1)` returns NaN. That is a reasonable result, but the student should be able to say why.

### Optimized solution

No further optimizations beyond the readable versions; the implementations are already tight. The key "optimization" is in `Mean`: explicitly converting to `float64` before accumulating prevents integer overflow. An `int` or `uint8` might overflow in the sum; converting each element to `float64` and then adding avoids this, at the cost of floating-point rounding.

### Trade-off

None significant. The readable versions are the optimized versions. The constraint definitions are the critical part; wrong constraints (missing `~`, wrong types) are the most common error here.

### Review checklist

- Are the constraints correctly defined with `~` on every base type?
- Does `Sum` work correctly with named types like `type Days int`? (Test: `Sum([]Days{1, 2, 3})`.)
- Does `SumBy` pass both the slice and an element-to-number function?
- Does `Mean` handle empty slices by returning the zero float and false? (Not returning a NaN or panicking.)
- Does `Clamp` panic if `lo > hi`, since that's a programmer error, not a runtime condition?
- Does `Clamp` validate `lo <= hi` with `cmp.Less` (NaN-aware) and use the builtin `min`/`max` rather than hand-written comparisons?
- Why does Clamp use `cmp.Ordered` rather than `Number`? (It only needs ordering, not arithmetic, so it can accept strings too. Choose the weakest constraint that works.)
- What is the student's experiment result from the package doc comment? (The doc should reference an experiment confirming that named types satisfy their underlying constraints.)

---

## Exercise 5: `iterx` — Lazy iterator adapters

### Readable solution

An `iter.Seq[T]` is a function type: `func(yield func(T) bool)`. The returned sequence is that function. `Filter` and `Map` compose: they return a sequence that ranges over the input sequence, applies a check or transformation, and yields kept elements.

The **one critical rule:** when `yield` returns `false` (the consumer has broken early), the iterator must return immediately and never call `yield` again. Every function checks this: `if !yield(v) { return }`.

`Naturals()` is infinite; callers must use `Take` or a loop with an explicit `break`. `Enumerate` returns `iter.Seq2` (key, value pairs; indices and values). Per-iteration state like `Take`'s `taken` counter lives inside the closure so each range starts fresh.

### Optimized solution

`FilterMap` fuses `Filter` and `Map` into one adapter: one closure layer and one `yield` call per kept element instead of two. This halves the overhead. Use it only when profiling shows the pipeline itself is hot; composing `Filter(Map(...))` reads better and the overhead is usually small.

**Measured on Go 1.24.2 linux/amd64:** Sum of squares of even numbers in a 10,000-element slice, three ways:
- Pipeline (Filter then Map): ~42,600 ns/op (280 B, 12 allocs)
- FilterMap (fused): ~36,300 ns/op (216 B, 8 allocs)
- Plain loop: ~7,500 ns/op (0 B, 0 allocs)
- **Speedup: FilterMap vs pipeline is ~15% faster.** The pipeline overhead is real (yield calls) but small compared to the plain loop (both are much slower because iterators involve closures and function calls). FilterMap is a win when you're already in hot code, but most code can stay with the composed version.

### Trade-off

- **Composed vs FilterMap:** Composed reads better and is easier to reason about. Use FilterMap only if you've profiled and found the iterator pipeline is your bottleneck. In most applications, the clarity of `Filter(Map(...))` is worth the extra closure call.

### Review checklist

- Does every iterator call `yield` and check its return value immediately? E.g., `if !yield(v) { return }`, not `yield(v)` with the return ignored. This is critical: ignoring the return triggers a runtime panic when the consumer breaks.
- Does `Take` declare its counter inside the closure, so each `range` starts fresh at 0? (Otherwise the second `range` over the same `Take` sequence will continue from where the first left off.)
- Does the iterator stop yielding as soon as the consumer signals it? (Test: `break` in a loop should exit early, pulling only as many elements as needed.)
- For `Naturals`: does the test confirm early break works? It should pull only as many elements as the consumer asks for.
- For `Enumerate` and `Filter` → `Map` composition: do the tests confirm reusability? The same `Seq` can be ranged twice and should yield the same elements both times.
- Are the return-value checks for `yield` consistent across all iterators?

---

## Project: Generic LRU Cache

### Overview

Build `Cache[K comparable, V any]` and an optional faster variant `FastCache`. Requirements: O(1) `Get`/`Put`/`Delete`, capacity-based eviction, optional TTL with an injectable clock, and an `All()` iterator in recency order. The spec, hints and stretch goals are in `project/README.md`.

### Readable solution

`Cache[K comparable, V any]` uses `container/list` for recency order (doubly-linked list) and a map from key to `*list.Element` for O(1) lookup. `Get` moves an entry to the front (most recently used) and returns it. `Put` adds or updates; if the cache is full, it evicts the back (least recently used) element before adding a new one.

Optional TTL: entries can expire. `Get` checks expiry and removes if expired. An `options` struct holds the TTL duration and a `now` function for injectable time in tests (critical for deterministic testing of expiry).

`All()` returns an `iter.Seq2[K, V]` in recency order, skipping expired entries. The container/list approach is readable, obviously correct, and easy to verify. The cost: allocates an `Element` per entry and boxes the entry value in `Element.Value.(any)`.

### Optimized solution

`FastCache[K comparable, V any]` lives entirely in a **preallocated nodes slice** (`make([]node[K, V], 1, capacity+1)`: the sentinel plus one slot per entry, never regrown). Nodes are referenced by int32 indices, not pointers, so the slice has no internal pointers for the GC to chase (unless K or V contain pointers). The TTL is stored as `int64 UnixNano` (8 bytes) instead of `time.Time` (24 bytes due to the `*Location` field). The clock is only consulted when TTL is configured, avoiding the clock call entirely otherwise.

All nodes sit contiguously in memory. When full, the evicted slot is reused for the new entry. Steady-state `Get`/`Put` allocate nothing. Trade-off: you maintain a hand-rolled doubly-linked list with a sentinel node, indexed by int32. Off-by-one bugs in the ring manipulation are easy to introduce.

**Measured on Go 1.24.2 linux/amd64:** 1000-capacity cache, 4096-iteration workload with skewed key access (roughly 2 Gets per Put), mixed hits, misses and evictions:
- Cache (list-based): ~84 ns/op (26 B/op, 0 allocs rounded; under one allocation per op)
- FastCache (arena): ~27.3 ns/op (0 B/op, 0 allocs)
- **Speedup: ~3x faster** on the mixed workload. Without TTL, the difference is clear. With TTL:
- Cache (list+TTL): ~96 ns/op
- FastCache (arena+TTL): ~63 ns/op
- **Speedup: ~1.5x faster** when TTL is enabled. The TTL check adds overhead to both; FastCache avoids boxing and element allocation, but the clock call cost is similar.

The no-allocation property of FastCache is the real win in latency-sensitive code: fewer allocations mean less GC work, which shows up in tail latency more than in average ns/op.

### Trade-off

- **Readable vs FastCache:** Use Cache unless you're building a production cache under sustained load. The list-based clarity is worth the 2–3x throughput difference in most applications. FastCache is valuable for:
  - Embedded systems or latency-critical services (e.g., DNS cache, TCP connection pool).
  - Learning how arena-based allocation and pointer-free structures work (educational value).
  - Code that profiles and confirms the cache is a bottleneck.

### Review checklist

**Readable (container/list-based):**
- Does `New` panic if capacity < 1? (A programming error.)
- Does `Get` move the entry to the front before returning it?
- Does `Get` check expiry and remove expired entries? If expired, return the zero V and false.
- Does `Put` evict the back element (LRU) if the cache is full before adding a new one?
- Does `Delete` correctly remove an entry and its map entry?
- Does `All()` return an iterator that respects expiry and yields in recency order?
- Is the clock injected via `WithClock` for testability? (Tests use a fake clock to control time.)
- For TTL: does `Put` check and set expiry correctly? Is the duration added to now correctly?

**FastCache (arena-based):**
- Are node links (prev/next) stored as int32 indices, with `0` as the sentinel?
- Does `NewFast` initialize the nodes slice with capacity 1 (the sentinel)? `make([]node[K, V], 1, capacity+1)`
- Does `nowNano()` avoid calling the clock when TTL is not configured? (Return 0 if ttl == 0.)
- Does `Get` update recency (by unlinking and pushing to front) only when the entry is not already at the front? (Optimization to avoid redundant work.)
- When evicting and the cache is full, is the back element's slot reused? Or does `Put` allocate a new slot?
- Does the `remove` function keep the arena dense by moving the last node into the hole?
- Does `All()` iterate in recency order and skip expired entries?
- Are there off-by-one bugs in the circular-list manipulation? (Hand-verify: `pushFront`, `unlink`, `remove`.)

### Common student errors

- **Off-by-one in FastCache indices:** Using `i+1` instead of the actual next index, or confusing the sentinel with a regular node.
- **Forgetting to check `yield`'s return value in iterators:** This causes a panic when a consumer breaks early.
- **Not injecting the clock:** Making `Get` and `Put` call `time.Now()` directly, making tests non-deterministic.
- **Forgetting TTL checks:** Returning expired entries, or never evicting them.
- **Allocating unnecessarily:** Creating new Elements or boxing values in FastCache.

---

## Summary of Measured Speedups (Go 1.24.2 linux/amd64, `-count=5`, medians, rounded)

| Component | Readable | Optimized | Speedup | Notes |
|---|---|---|---|---|
| Map | ~48,500 ns/op | ~45,400 ns/op | ~5–10% (noise-level) | Index assignment vs `append` |
| Filter | ~76,700 ns/op (16 allocs) | ~60,700 ns/op (1 alloc) | ~20–35% | Preallocation vs growth |
| FilterInPlace | ~76,700 ns/op | ~23,400 ns/op | ~3x | Zero-alloc, reuses caller's array |
| Intersect | ~2,470,000 ns/op | ~4,100 ns/op | ~600x | Algorithm change (iterate smaller set) |
| Queue churn | ~12.3 ns/op | ~2.9 ns/op (RingQueue) | ~4x | No periodic regrow/copy |
| FilterMap | ~42,600 ns/op (pipeline) | ~36,300 ns/op | ~15% | Fusion of two closures |
| Cache | ~84 ns/op (list) | ~27.3 ns/op (arena) | ~3x | Arena allocation, no boxing |
| Cache+TTL | ~96 ns/op | ~63 ns/op | ~1.5x | TTL check overhead |

## Key Implementation Patterns

### Type Constraints and ~

The `~` operator in constraints is essential when working with named types:

```go
type Days int

// Without ~: Days does not satisfy int
func Sum[T int](xs []T) T { ... }
Sum([]Days{1, 2}) // compile error

// With ~: Days satisfies ~int
func Sum[T ~int](xs []T) T { ... }
Sum([]Days{1, 2}) // OK
```

Always include `~` when defining numeric constraints. The pattern `S ~[]E` is ubiquitous in slices to preserve the caller's type. For example, `type IntList []int; Filter(myIntList, ...)` returns `IntList`, not `[]int`.

### Zero Values and Nil Safety

Go's zero value semantics apply to generic types:

- A nil map read doesn't panic; only writes do. Use this in Set/Cache implementations: read operations handle nil receivers, write operations initialize on first call.
- The zero value of a type parameter `T` is always valid. For Stack/Queue, use `var zero T` to clear slots after popping.
- Slices and maps can be nil; handle this explicitly in iterators and operations. `Union(nil, Set)` should return a non-nil Set with just the right elements, not nil.

### Iterator Correctness

Iterators have one non-negotiable rule: **check `yield`'s return value immediately and return if false**. Pattern:

```go
// Correct: check and return
for elem := range input {
    if !yield(transform(elem)) {
        return
    }
}

// WRONG: ignores yield's return value, causes panic on early break
for elem := range input {
    yield(transform(elem))
}
```

The `yield` function returns false when the caller breaks early. Ignoring this causes a panic the moment a consumer calls `break`. Write tests that explicitly break:

```go
for i, v := range Take(Naturals(), 5) {
    if i == 2 { break } // tests that early stop works
}
```

### Reusability of Sequences

A `iter.Seq[T]` can be ranged multiple times if state is managed correctly:

- **Stateful sequences** (like `Take` or a counter-based sequence) must declare state inside the closure so each range starts fresh.
- **Stateless sequences** (like `Filter` over a slice) can be ranged any number of times.

Test both:

```go
seq := Filter(data, predicate)
first := slices.Collect(seq)
second := slices.Collect(seq)
// first and second should be equal
```

### Efficient Slice Handling

When slices are passed to generic functions, understand the backing array:

- `Filter` that does `out := s[:0]; append(out, ...)` **destroys s** by overwriting its data.
- `FilterFast` with pre-allocation doesn't touch s.
- Only use in-place techniques when you own the slice entirely and never read it again.

---

## Testing Patterns and Edge Cases

### Nil and Empty Handling

Test every function with:
- Nil inputs (slices, maps): `Map(nil, f)`, `Set.All() over an empty set`
- Empty slices: `Filter([]int{}, keep)`
- Empty maps: `Union(nil, nil)`

Nil and empty are distinct in Go: `nil` is the zero value, `[]int{}` is an allocated but zero-length slice. Both should be handled.

### Iterator Early Termination

Always write tests that break from the iterator loop:

```go
for v := range Filter(input, predicate) {
    if someCondition { break }
}
```

This catches the bug where `yield` return value is ignored.

### Concurrency (Not in 06-generics, but mentioned for awareness)

Generic types are **not inherently thread-safe**. A `Set[int]` shared between goroutines needs external synchronization. If a later module covers concurrency, note that sharing `Cache` or `Set` requires a mutex.

---

## Common Student Mistakes and How to Grade Them

### 1. Ignoring `yield` return value in iterators

```go
// WRONG: will panic on early break
for v := range seq {
    yield(f(v))  // ignores return value
}
```

**Feedback:** Iterators must check `yield` and return immediately when false. Run `go test -run TestTake -v` to see the panic. Add `if !yield(...) { return }`.

### 2. Destroying input in Filter

```go
// WRONG: Filter reuses s's backing array
func Filter[S ~[]E, E any](s S, keep func(E) bool) S {
    out := s[:0]  // this overwrites s!
    for _, v := range s {
        if keep(v) {
            out = append(out, v)
        }
    }
    return out
}
```

**Feedback:** The caller's slice is modified. Create a fresh slice: `var out S` (nil slice). This is a correctness bug, not a performance issue.

### 3. Missing `~` in constraint definitions

```go
// WRONG: type Days int does not satisfy int
func Sum[T int](xs []T) T { ... }

// CORRECT: Days satisfies ~int
func Sum[T ~int](xs []T) T { ... }
```

**Feedback:** When a constraint is a single type (not a union), add `~` if you want named types to satisfy it.

### 4. Off-by-one in FastCache/RingQueue

Hand-verify the circular-list logic:
- Does `pushFront` correctly set prev/next?
- Does `unlink` update neighbors correctly?
- In a ring, does wrapping use `& (len(buf) - 1)` correctly?

**Feedback:** Run the tests with different capacities and workloads. Add print debugging inside the ring logic if tests fail.

### 5. Not checking expiry in Cache.All()

The All() iterator must skip expired entries. If TTL is set, the iterator should call `isExpired` before yielding.

**Feedback:** Test with a fake clock: add entries with TTL, advance time, then iterate and confirm expired entries are skipped.

### 6. Panicking on nil in Set/Stack/Queue operations

`Contains` on a nil Set should return false, not panic. `Pop` on an empty Stack should return false.

**Feedback:** Add checks: `if s == nil || s.m == nil { ... }` at the start of read methods.

### 7. Returning nil instead of empty map/slice

```go
// WRONG: returns nil
func Union[T comparable](a, b *Set[T]) *Set[T] {
    if a == nil && b == nil {
        return nil  // JSON marshals differently than {}
    }
    ...
}

// CORRECT: returns non-nil empty
func Union[T comparable](a, b *Set[T]) *Set[T] {
    result := &Set[T]{m: make(map[T]struct{})}
    // ... add elements ...
    return result  // guaranteed non-nil
}
```

**Feedback:** For JSON-compatible code, always return non-nil empty collections. This is a subtle but important contract.

---

## Grading Summary

### Exercise 1: funcs
- **Readable solutions (70%):** Correct Map, Filter, Reduce, GroupBy.
- **Optimized variants (20%):** Presence of fast.go and benchmarks; understanding trade-offs.
- **Quality (10%):** No doc comments required but appreciated; explanation of why MapFast helps.

### Exercise 2: set
- **Readable Set (60%):** Correct Add/Remove/Contains/Len/All, non-nil returns, iterator correctness.
- **Set operations (20%):** Union/Intersect/Difference correct; Sorted as a function, not a method.
- **Quality (20%):** Nil-safe reads; IntersectFast optimization (optional but impressive).

### Exercise 3: stackqueue
- **Stack and Queue (70%):** Correct Push/Pop/Peek/Len; zero-value readiness; slot zeroing.
- **RingQueue optimization (20%):** Correct circular-buffer logic (if attempted); no off-by-one bugs.
- **Quality (10%):** Comments explaining the steady-state alloc difference.

### Exercise 4: numeric
- **Constraints and functions (80%):** Sum/SumBy/Mean/Clamp with correct constraints; `~` used correctly.
- **Edge cases (10%):** Empty-slice handling; NaN handling in Clamp.
- **Quality (10%):** Experiment result mentioned in doc comment; explanation of why Mean returns bool.

### Exercise 5: iterx
- **Basic iterators (60%):** Filter/Map/Take/Enumerate/Naturals; correct yield checking.
- **Composition (20%):** Laziness confirmed; reusability over same sequence; early break works.
- **FilterMap optimization (20%):** Present; understanding when to use.

### Project: LRU Cache
- **Readable Cache (60%):** Correct Get/Put/Delete; LRU eviction; TTL and clock injection.
- **Iterator (15%):** All() in recency order; expiry handling.
- **FastCache optimization (20%):** Arena allocation; steady-state zero allocs; correct sentinel manipulation; optional but highly valued.
- **Quality (5%):** Tests pass; no panics on edge cases; doc comments present.

---

## What changed after Go 1.24

Go 1.27 (released after this course was written) added **generic methods**: a method can now declare its own type parameters. So `func (l List[T]) Map[U any](f func(T) U) List[U]` compiles in 1.27+. In Go 1.24, the workaround is a top-level function like `func MapList[T, U any](l List[T], f func(T) U) List[U]`. You'll see the function form throughout older code, and this course uses it. Mention generic methods in prose when teaching 1.24 code, but do not use them in the 06-generics solutions.
