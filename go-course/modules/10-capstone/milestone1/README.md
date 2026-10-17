# Milestone 1: In-Memory Store with TTL

**Time budget:** ≤ 60 minutes

## Overview

Implement a concurrency-safe, in-memory key-value store with per-key time-to-live (TTL) expiry. Entries expire at an absolute instant; the store enforces expiry lazily (on read) and actively (via a background janitor goroutine).

## Specification

### API

```go
// Store is a concurrency-safe in-memory key-value store.
type Store struct { ... }

// Create a new store.
func New(opts ...Option) *Store

// Set stores value under key, replacing any previous value and TTL.
// If ttl > 0, the entry expires ttl after now; otherwise it never expires.
func (s *Store) Set(key, value string, ttl time.Duration)

// Get returns the value stored under key.
// It returns ErrNotFound if the key is absent or expired.
func (s *Store) Get(key string) (string, error)

// Delete removes key and reports whether a live entry was removed.
// An expired entry is removed, but Delete returns false.
func (s *Store) Delete(key string) bool

// Keys returns the live (unexpired) keys in ascending order.
// The returned slice is owned by the caller.
func (s *Store) Keys() []string

// Len returns the number of live entries.
func (s *Store) Len() int

// DeleteExpired removes every expired entry and returns how many.
func (s *Store) DeleteExpired() int

// RunJanitor calls DeleteExpired every interval until ctx is canceled.
// If interval <= 0, returns immediately (janitor disabled).
func (s *Store) RunJanitor(ctx context.Context, interval time.Duration)

// janitor runs one sweep per tick and returns when ctx is canceled.
// Separating it from RunJanitor lets tests drive sweeps with a fake clock.
func (s *Store) janitor(ctx context.Context, ticks <-chan time.Time)
```

### Behavior

**TTL:**
- Entries are expired at an absolute instant, calculated as `now + ttl`.
- An entry whose expiry equals the current instant is expired (e.g., a 10-second TTL means "gone at +10s").
- A TTL of 0 or negative means the entry never expires.

**Expiry enforcement:**
1. **Lazy**: When you call Get, Keys, Len, or Delete, they treat expired entries as absent.
2. **Active**: RunJanitor starts a background goroutine that calls DeleteExpired every interval to reclaim memory.

**Concurrency:**
- All operations (Get, Set, Delete, Keys, Len, DeleteExpired) are safe for concurrent use.
- Use sync.Mutex or sync.RWMutex to guard the items map.
- The janitor runs in its own goroutine and must safely check the clock, even while the main thread advances a fake clock (see Testing).

**Clock injection:**
- The store reads "now" via a `func() time.Time` field.
- By default, this is `time.Now`.
- Tests can override it with `WithClock(fakeClockFunc)` to control time without sleeping.

### Testing

You don't write tests for this milestone; they're provided. However, understand the test strategy:

- **Fake clock**: Tests inject a controllable clock (see the test's `fakeClock` type). This lets you advance time instantly without sleeping.
- **No sleeps**: The provided tests never call `time.Sleep`. They advance the clock and check state immediately.
- **Janitor control**: Tests either:
  - Call `RunJanitor` with a real ticker to verify it works.
  - Call `janitor` directly with a manual channel to drive it tick-by-tick.
- **Concurrency testing**: `TestConcurrentAccess` hammers the store from 8 goroutines while the janitor runs. It only detects bugs under `go test -race`.

### Run the tests

```bash
cd /home/csmith/Workspace/tools/go-course
go test ./modules/10-capstone/milestone1/... -race
```

The tests should **fail** until you implement the TODOs. When done, all tests pass with `-race`.

---

## Implementation Guide

### Design Decisions

**Why RWMutex?** The store has many readers (Get, Keys, Len) and fewer writers (Set, Delete, RunJanitor). RWMutex allows readers to run in parallel, only blocking on writes.

**Why two-level expiry?** Lazy expiry means reads never stall waiting for cleanup. Active expiry reclaims memory so the store doesn't grow unbounded. Together, they're robust.

**Why a janitor?** If entries were never cleaned up, the map would grow forever, even if every key had expired. The janitor removes stale entries periodically.

**Why a separate janitor function?** Tests need to drive sweeps without waiting for a ticker. By separating `janitor(ctx, ticks)` from `RunJanitor(ctx, interval)`, tests can pass a channel they control.

### Step-by-step

1. **Implement `expired`**: Check if an entry is expired at a given instant.
   ```go
   func (e entry) expired(now time.Time) bool {
       // An entry with expiresAt == 0 never expires.
       // Otherwise, it's expired if now >= expiresAt.
       // Use time.Time.IsZero and Before.
   }
   ```

2. **Implement `Set`**:
   - Lock the mutex (write lock or full lock).
   - Calculate `expiresAt` as `now().Add(ttl)` if ttl > 0, otherwise zero.
   - Store the entry.

3. **Implement `Get`**:
   - Read-lock the mutex.
   - Fetch the entry; if absent or expired, return ErrNotFound.
   - Otherwise, return the value.

4. **Implement `Delete`**:
   - Lock the mutex.
   - Fetch the entry; if absent, return false.
   - Delete it; return true if it was not expired, false if it was already expired.

5. **Implement `Keys`**:
   - Read-lock, collect live keys into a slice.
   - Release the lock.
   - Sort the slice *outside* the lock.
   - Return it.

6. **Implement `Len`**:
   - Read-lock the mutex.
   - Count live entries.
   - Release and return.

7. **Implement `DeleteExpired`**:
   - Lock the mutex (write lock or full lock).
   - Iterate the map, delete expired entries, count them.
   - Release and return the count.

8. **Implement `RunJanitor`**:
   - If `interval <= 0`, return immediately (janitor disabled).
   - Create a time.Ticker.
   - Defer `t.Stop()` to clean up (crucial!).
   - Call `janitor(ctx, t.C)`.

9. **Implement `janitor`**:
   - Loop forever.
   - Select on `ctx.Done()` and `ticks`.
   - When ctx is canceled, return.
   - When a tick arrives, call `DeleteExpired()`.

---

## Common Mistakes

1. **Forgetting to Stop the Ticker**: A stopped Ticker can still deliver ticks. Forgetting `defer t.Stop()` causes goroutine leaks.

2. **Holding the lock too long**: Don't hold the lock while sorting or doing expensive work. Read the data, release the lock, then process.

3. **Not handling the zero time**: A zero `time.Time` means "never expires". Check with `IsZero()`, not `== time.Time{}`.

4. **Confusing before and after**: Expiry at time T means the entry is gone at T, not after. So `now.Before(expiresAt)` is alive, `!now.Before(expiresAt)` is expired.

5. **Race conditions on the clock**: Tests advance the clock while the janitor reads it. Make sure your clock function is thread-safe (tests provide a safe one).

6. **Leaking goroutines**: If janitor doesn't return when ctx is canceled, the test will hang. Always check `ctx.Done()` in the select loop.

---

## Rubric (for instructor review)

When reviewing a student's solution, check:

- ✓ All functions compile and tests pass with `-race`.
- ✓ `expired()` correctly handles zero and non-zero times.
- ✓ `Set` and `Get` handle TTL correctly (entries expire at the right instant).
- ✓ `Keys` returns a sorted slice and doesn't corrupt the map when values are mutated externally.
- ✓ `DeleteExpired` is called by janitor and removes stale entries.
- ✓ `RunJanitor` stops the ticker and returns when ctx is canceled.
- ✓ Janitor doesn't leak goroutines (no hanging on cancellation).
- ✓ `-race` detector finds no issues under `TestConcurrentAccess`.
- ✓ No sleeps in the code (real clock only in `time.Now`).

---

## Stretch Goals (Optional)

1. **Measure lock contention.** Write a benchmark with `b.RunParallel` doing
   90% `Get` / 10% `Set` over 1024 keys, and run it with `-cpu=1,4,8`. Does
   ns/op go *up* as you add cores? That is lock contention.
2. **Shard the store.** Build a `ShardedStore` with N independently locked
   maps, picking the shard by hashing the key. Write a differential test
   that runs the same operations on `Store` and `ShardedStore` and compares
   results. Then re-run the benchmark: when does sharding win, and when does
   it lose? (Hint: try `-cpu=1`.)
3. **Make `Keys` cheaper.** Can it avoid holding the lock while sorting?
   Measure with `-benchmem`.

After your attempt, compare with `solutions/10-capstone/milestone1/store/fast.go`
and the measured numbers in the instructor's review.
