# Capstone Module Review & Rubric

This document contains instructor notes for reviewing student solutions to the capstone milestones.

---

## Milestone 1: In-Memory Store with TTL

### Solution Overview

The readable solution (store.go) is the recommended implementation:
- Uses `sync.RWMutex` for readers/writers.
- Lazy expiry on every read; active cleanup via janitor.
- Separates `janitor(ctx, ticks)` from `RunJanitor(ctx, interval)` for testability.

### Readable Solution

See **solutions/10-capstone/milestone1/store/store.go**. Key design:

```go
type entry struct {
    value     string
    expiresAt time.Time  // zero = never expires
}

func (e entry) expired(now time.Time) bool {
    return !e.expiresAt.IsZero() && !now.Before(e.expiresAt)
}

// Store Guards items with RWMutex. Readers use RLock; writers use Lock.

func (s *Store) Get(key string) (string, error) {
    now := s.now()  // call s.now() every operation
    s.mu.RLock()
    e, ok := s.items[key]
    s.mu.RUnlock()
    if !ok || e.expired(now) {
        return "", ErrNotFound
    }
    return e.value, nil
}

func (s *Store) Keys() []string {
    now := s.now()
    s.mu.RLock()
    // Collect live keys
    keys := make([]string, 0, len(s.items))
    for k, e := range s.items {
        if !e.expired(now) {
            keys = append(keys, k)
        }
    }
    s.mu.RUnlock()
    // Sort outside the lock
    slices.Sort(keys)
    return keys
}

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
```

### Optimized Solution

See **solutions/10-capstone/milestone1/store/fast.go**: `ShardedStore`.

The readable `Store` has one `sync.RWMutex` for the whole map, so every
`Set` blocks every other goroutine, and even concurrent `Get`s contend on
the RWMutex's reader counter. `ShardedStore` splits the key space into 32
shards, each with its own lock and map:

- **Shard choice** is an inline FNV-1a hash of the key, masked with
  `shardCount-1` (a power of two). It is written out by hand because
  `hash/fnv` would need a `[]byte` conversion and an interface call.
- **Padding**: each shard is padded to 128 bytes so two shards' mutexes never
  share a cache line. Without it, cores locking *different* shards would
  still invalidate each other's cache line (false sharing).
- `Get`/`Set`/`Delete` touch one shard. `Keys`, `Len` and `DeleteExpired` visit
  all 32, which makes them slower and **no longer a single atomic snapshot**.
- `TestShardedStoreMatchesStore` checks it against the readable `Store`
  (a differential test).

**Measured on go1.24.2 linux/amd64** (`BenchmarkMixed*`: 1024 keys, 90%
`Get` / 10% `Set` from `GOMAXPROCS` goroutines via `b.RunParallel`; medians of 5;
0 allocs/op for both):

| GOMAXPROCS (`-cpu`) | `Store` ns/op | `ShardedStore` ns/op | Speedup |
|---|---|---|---|
| 1 | ~65 | ~70 | 0.93× (sharded is *slower*: hashing, no contention to remove) |
| 4 | ~207 | ~80 | ~2.6× |
| 20 | ~264 | ~60 | ~4.4× |

Note how the readable store gets *slower* per op as cores are added (lock
contention), while the sharded one stays flat. That is the signature of a
contended lock in a benchmark, and a good thing to show the student.

```bash
go test -run='^$' -bench=Mixed -benchmem -cpu=1,4,20 ./solutions/10-capstone/milestone1/store/
```

**When to use the sharded version:**
- Many goroutines hit the store concurrently (a server under load), and a
  profile (`go test -mutexprofile`, or `runtime/pprof` mutex profile)
  shows time waiting on the store's lock.

**When to stick with the readable version:**
- Single-threaded or lightly concurrent use. It is faster there.
- When `Keys`/`Len` must be a consistent snapshot.
- Always as the first version: shard only after measuring contention.

### Trade-offs

| Aspect | Readable `Store` | `ShardedStore` |
|--------|----------|------|
| **Complexity** | One lock, one map | 32 locks, hashing, padding |
| **Single goroutine** | Faster (~65 vs ~70 ns/op) | Pays for hashing |
| **Many goroutines** | Contends on one lock | Scales with cores |
| **`Keys`/`Len`** | Consistent snapshot | Visits 32 shards; not atomic across them |
| **Memory** | One map | 32 maps + 4 KiB of padded shard headers |

### Grading Checklist (Milestone 1)

For each student submission:

- ✓ **Correctness**: All tests pass with `-race`. No timeouts or hangs.
- ✓ **TTL semantics**: Keys expire at the right instant (not before, not after). Zero TTL never expires.
- ✓ **Lazy expiry**: Get and Keys treat expired entries as absent without calling DeleteExpired first.
- ✓ **Active expiry**: DeleteExpired is called by janitor; expired entries are removed.
- ✓ **Concurrency**: `-race` detector reports no issues. Multiple readers can run in parallel (RWMutex).
- ✓ **Clock injection**: Uses `s.now()`, not `time.Now()` directly. Tests can inject a fake clock.
- ✓ **Goroutine lifecycle**: Janitor stops cleanly when context is canceled. No leaks under TestConcurrentAccess.
- ✓ **No sleeps**: Code doesn't call `time.Sleep` (real clock only in default initialization).
- ✓ **Lock discipline**: Locks are held for minimal time. Sorting (in Keys) happens outside the lock.

**Common issues to watch:**

1. **Using Mutex instead of RWMutex**: Works but prevents parallelism. Accept if correctness is demonstrated.

2. **Not checking IsZero()**: Tests will fail if zero-TTL entries don't expire correctly.

3. **Holding lock during sort**: Blocks readers. Point out; recommend moving sort outside the critical section.

4. **Not stopping the Ticker**: Goroutine leak. Critical issue; must be fixed.

5. **Blocking in janitor select**: If the janitor doesn't properly handle context cancellation, tests hang. Critical.

6. **Race condition on clock**: Fake clock must be thread-safe (e.g., using a mutex). The provided test clock is correct; check student's code if they write their own.

---

## Milestone 2: Append-Only Log & Persistence

### Solution Overview

Three files:

1. **memory.go**: Milestone 1 Store + SetExpiresAt + Items.
2. **log.go**: Append-only log with Replay and Compact.
3. **db.go**: Wrapper that serializes writes to the log.

### Readable Solution

See **solutions/10-capstone/milestone2/store/\*.go**. Key design:

**Log format (JSON Lines):**
```json
{"op":"set","key":"k","value":"v","exp":12345}
{"op":"del","key":"k"}
```

**One write per record** ensures crashes tear at most the final record:
```go
func (l *Log) Append(r Record) error {
    b, err := encodeRecord(r)
    l.mu.Lock()
    defer l.mu.Unlock()
    if _, err := l.f.Write(b); err != nil {  // single Write call
        return err
    }
    if l.sync {
        return l.f.Sync()
    }
    return nil
}
```

**Replay handles torn records:**
```go
func (l *Log) Replay(fn func(Record) error) error {
    br := bufio.NewReader(l.f)
    var good int64
    for lineNo := 1; ; lineNo++ {
        line, err := br.ReadBytes('\n')
        if err == io.EOF {
            if len(line) > 0 {  // torn tail
                l.f.Truncate(good)
            }
            return nil
        }
        // Process line
        good += int64(len(line))
    }
}
```

**Atomic Compact:**
```go
func (l *Log) Compact(records []Record) error {
    // 1. Write to temp file (bufio.NewWriter + Flush + Sync)
    tmpName := writeTemp(dir, pattern, records)
    // 2. Atomic rename
    os.Rename(tmpName, l.path)
    // 3. Fsync directory (make rename durable)
    syncDir(dir)
    // 4. Reopen with O_APPEND
    l.f = os.OpenFile(l.path, os.O_RDWR|os.O_APPEND, ...)
}
```

**DB serializes writes:**
```go
func (db *DB) Set(key, value string, ttl time.Duration) error {
    db.mu.Lock()
    defer db.mu.Unlock()
    // Write-ahead: log first, then apply
    rec := Record{Op: OpSet, Key: key, Value: value, ExpiresAt: toUnixNano(expiresAt)}
    if err := db.log.Append(rec); err != nil {
        return err  // memory unchanged
    }
    db.mem.SetExpiresAt(key, value, expiresAt)
    return nil
}
```

### Optimized Solution

Optimizations for Milestone 2 (listed for reference; readable solution is generally sufficient):

1. **Batch compaction**: Instead of compacting every N writes, batch them (e.g., compact after log grows to 10MB).
2. **Incremental checkpoints**: Instead of rewriting the entire log, append a checkpoint record periodically.
3. **Lazy fsync**: For non-critical writes, skip fsync and rely on OS buffer; for critical, fsync.
4. **Buffer writes**: Batch multiple Records into one write() call (bufio helps).

**None of these are required for the capstone.** The readable solution is clear and efficient for reasonable dataset sizes.

**What durability costs, measured** (`bench_test.go`, solutions only; go1.24.2
linux/amd64, ext4 on an SSD, medians of 3):

| `BenchmarkAppend` | ns/op | allocs/op |
|---|---|---|
| `nosync` (`OpenLog(path, false)`) | ~690 | 2 |
| `fsync` (`OpenLog(path, true)`) | ~1,176,000 (≈1.2 ms) | 2 |

That is about 1,700× per write. This one number explains why every optimization
above revolves around fsync: *group commit* (many writers share one fsync)
and *lazy fsync* (fsync every N ms, accepting the loss of that window) exist
because the disk flush, not the encoding or the `write` syscall, dominates.
A good review question for the student: "with `syncWrites=false`, which
acknowledged writes can a power cut lose? And a `kill -9`?" (Power cut: anything
not yet flushed from the page cache. `kill -9`: nothing that `Write` returned
for, because the kernel still has it.)

```bash
go test ./solutions/10-capstone/milestone2/store/ -run='^$' -bench=Append -benchmem
```

### Trade-offs

| Aspect | Readable | Optimized |
|--------|----------|-----------|
| **Simplicity** | Clear, easy to understand | Complex batching/checkpointing logic |
| **Durability** | Guaranteed after fsync | Depends on batch policy |
| **Performance** | Fast for small datasets | Better for high-write workloads |
| **Recovery time** | Replay entire log | Faster with checkpoints |

### Grading Checklist (Milestone 2)

- ✓ **Correctness**: All tests pass with `-race`. Data survives a simulated restart (using t.TempDir).
- ✓ **WAL semantics**: Log write completes before memory update. If log fails, memory is unchanged.
- ✓ **Torn write handling**: Replay detects incomplete final records and truncates.
- ✓ **Compaction**: Produces a minimal log with one record per live key. Atomic (no partial renames).
- ✓ **Concurrency**: Writes are serialized (mutex). Reads served from memory without touching disk.
- ✓ **Record format**: Valid JSON, one per line, can be read/written correctly.
- ✓ **Fsync usage**: If Options.Sync is true, every write fsyncs. If false, it doesn't.
- ✓ **Error handling**: Failed log appends don't corrupt state. Corrupt records yield errors.
- ✓ **Cleanup**: Files are closed properly. Log.Close is idempotent.

**Common issues:**

1. **Not truncating the log after a torn record**: Subsequent replays will error on garbage.

2. **Forgetting to reopen after Compact**: Old file descriptor points to an unlinked inode; writes are lost.

3. **Not fsyncing the directory after rename**: A power loss before dir fsync might revert the compaction.

4. **Not serializing writes**: Two concurrent Set calls might interleave their log writes, corrupting the log.

5. **Panicking on JSON errors**: Wrap errors and return them; don't panic.

6. **Off-by-one in truncate offset**: Track the position of the last complete record carefully.

---

## Milestone 3: HTTP API & Graceful Shutdown

### Solution Overview

Two parts:

1. **api.go**: HTTP handlers (GET, PUT, DELETE).
2. **cmd/kvd/main.go**: Runnable server with graceful shutdown.

### Readable Solution

See **solutions/10-capstone/milestone3/store/api.go** and **cmd/kvd/main.go**. Key design:

**Server struct:**
```go
type Server struct {
    db *DB
    lg *slog.Logger
}

func NewServer(db *DB, lg *slog.Logger) *Server {
    return &Server{db: db, lg: lg}
}

func (s *Server) Register(mux *http.ServeMux) {
    mux.HandleFunc("GET /kv/{key}", s.get)
    mux.HandleFunc("PUT /kv/{key}", s.put)
    mux.HandleFunc("DELETE /kv/{key}", s.delete)
}
```

**Handlers (example: get):**
```go
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
    key := r.PathValue("key")
    if key == "" {
        s.writeJSON(w, http.StatusBadRequest, errorResp{Error: "key required"})
        s.lg.InfoContext(r.Context(), "get", "key", key, "status", 400)
        return
    }
    v, err := s.db.Get(key)
    if errors.Is(err, ErrNotFound) {
        s.writeJSON(w, http.StatusNotFound, errorResp{Error: "not found"})
        s.lg.InfoContext(r.Context(), "get", "key", key, "status", 404)
        return
    }
    if err != nil {
        s.writeJSON(w, http.StatusInternalServerError, errorResp{Error: "db error"})
        s.lg.ErrorContext(r.Context(), "get", "error", err)
        return
    }
    type resp struct { Value string `json:"value"` }
    s.writeJSON(w, http.StatusOK, resp{Value: v})
    s.lg.InfoContext(r.Context(), "get", "key", key, "status", 200)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if v != nil {
        json.NewEncoder(w).Encode(v)
    }
}
```

**Main program** (`cmd/kvd/main.go`): `main` only wires signals to a
context and calls `run(ctx, args, stdout, stderr) error`; everything else is
in `run`, which `cmd/kvd/main_test.go` drives end to end. Each line of the
reference exists to prevent a specific, common bug:

| Code in `run` | Bug it prevents |
|---|---|
| `signal.NotifyContext(..., os.Interrupt, syscall.SIGTERM)` (in `main`) | Ignoring SIGTERM: `docker stop`/Kubernetes/systemd kill the process after a grace period, without a clean shutdown |
| `os.Exit` only in `main`, after `run` returns | `os.Exit` skips deferred calls, so `defer db.Close()` silently never runs on error paths |
| deferred `db.Close()` whose error is joined into `run`'s result | Losing the one error that says buffered data never reached disk |
| `logger.Info("server started", "addr", ln.Addr().String())` | Logging the flag value instead of the real address (wrong for `:0`) |
| janitor in its own cancelable context + `<-janitorDone` before close | A background goroutine touching the DB after `Close` |
| `ReadHeaderTimeout` (and read/write/idle timeouts) | Slowloris: clients holding connections open forever |
| `select` on `serveErr` *and* `ctx.Done()` | Hanging forever if `Serve` fails (e.g. the listener dies) |
| `srv.Shutdown` with a 5s timeout, then `errors.Is(<-serveErr, http.ErrServerClosed)` | Dropping in-flight requests; mistaking the normal "server closed" result for a failure |

The defers run last-in-first-out: cancel the shutdown timeout, stop the
janitor and wait for it, then close the database. That is exactly the
required order.

### Optimized Solution

Not required, but worth discussing:

1. **Response encoding**: `writeJSON` encodes straight into the
   `ResponseWriter`. Module 08's `WriteJSONFast` (pooled buffer, shared
   header slice) measured ~1.5× faster than the readable version
   (~309 vs ~480 ns/op). Next to a network round trip, though, and next to
   an fsync of ~1.2 ms when `syncWrites` is on (see Milestone 2), that saving
   is invisible. **Measure where the time goes before optimizing the HTTP
   layer.** For this server the answer is the disk.
2. **Group commit** in the DB layer (many concurrent PUTs share one fsync)
   is the optimization that actually changes throughput under load.
3. **`ShardedStore`** from Milestone 1 under the DB removes lock contention
   for read-heavy traffic (measured ~4.4× at 20 cores for a 90% read mix).
4. **Middleware** for request logging, metrics (`GET /metrics`) and
   per-request timeouts: these are about operability, not speed.

### Trade-offs

| Aspect | Readable | Enhanced |
|--------|----------|----------|
| **Simplicity** | Simple handlers, inline logging | Middleware, more LOC |
| **Extensibility** | Hardcoded routes | Easier to add new routes/handlers |
| **Debugging** | Per-handler logs | Centralized logging |
| **Performance** | Direct, no overhead | Middleware overhead (usually small) |

For the capstone, stick with the readable solution. Middleware can be explored later.

### Grading Checklist (Milestone 3)

- ✓ **HTTP endpoints**: All three routes registered and respond correctly.
- ✓ **Status codes**: 200 on success, 404 on not found, 400 on bad request, 500 on error.
- ✓ **JSON format**: Responses are valid JSON with expected fields.
- ✓ **Path parameters**: `r.PathValue("key")` is extracted correctly. Empty key returns 400.
- ✓ **TTL handling**: PUT request body's TTL is in milliseconds; converted to time.Duration.
- ✓ **Expiry**: GET returns 404 for expired keys.
- ✓ **Request decoding**: Invalid JSON bodies return 400.
- ✓ **Logging**: All operations logged with slog (not fmt.Printf). Key, status, and error included.
- ✓ **Graceful shutdown**: On SIGINT **and SIGTERM** (check `syscall.SIGTERM` is passed to `signal.NotifyContext`), the server shuts down cleanly. No goroutine leaks.
- ✓ **Testable main**: `run(ctx, args, stdout, stderr) error`; no `os.Exit` or `log.Fatal` inside `run`; `cmd/kvd` tests pass.
- ✓ **Serve failure**: `run` returns if `Serve` fails, instead of waiting forever for a signal.
- ✓ **Server timeouts**: at least `ReadHeaderTimeout` is set.
- ✓ **Database close**: DB is closed on every return path (including errors after `Open`), after the janitor has stopped, and the `Close` error is reported.
- ✓ **Durability**: Restart the server; data persists.
- ✓ **Race-free**: `-race` detector reports no issues.

**Common issues:**

1. **Using fmt.Printf instead of slog**: Points for logging, but emphasize structured logging (slog).

2. **Not handling SIGINT/SIGTERM**: Server doesn't shut down cleanly.

3. **Not closing the database, or `defer db.Close()` followed by `os.Exit(1)`**: `os.Exit` skips defers. Here `Append` writes straight to the file, so the data is not lost on a clean exit. But the file is never closed, the error `Close` might report is never seen, and the pattern is a real data-loss bug with any buffered writer.

4. **Hardcoding listen address**: Should use the `-addr` flag.

5. **Not converting TTL units**: Milliseconds to time.Duration conversion missing or wrong.

6. **Panicking in handlers**: Return 500 and log the error instead.

7. **Not logging errors**: Operations that fail should be logged for debugging.

---

## Overall Module Rubric

After reviewing all three milestones:

| Criterion | Full | Partial | None |
|-----------|------|---------|------|
| **Correctness** | All tests pass. No race conditions. | Some edge cases fail. Minor race conditions. | Tests fail. Significant race conditions. |
| **Concurrency** | Proper mutex usage. No leaks. `-race` clean. | Correct but inefficient (e.g., holding locks too long). | Race conditions or deadlocks. |
| **Durability (M2)** | Torn writes handled. Compact is atomic. Data survives restart. | Some edge cases in crash recovery. | Incomplete or missing durability. |
| **HTTP API (M3)** | All endpoints work. Correct status codes. Graceful shutdown. | Some endpoints work. Status codes mostly correct. | Endpoints missing or non-functional. |
| **Code Quality** | Readable, idiomatic Go. Good error handling. | Mostly readable. Some anti-patterns (e.g., panic, global state). | Unreadable. Poor error handling. |
| **Documentation** | Clear comments. Types and functions documented. | Minimal comments. Some functions undocumented. | No comments. No documentation. |

---

## Feedback Template

Here's a template for instructor feedback:

```
## Milestone 1

✓ Passed all tests with -race.
✓ TTL logic is correct; zero TTL never expires.
✓ Janitor stops cleanly on context cancellation.
✓ Good use of RWMutex for reader parallelism.

Suggestions:
- Consider moving the sort outside the lock in Keys for even better concurrency.
- The fake clock in tests is a nice pattern; remember it for future projects.

## Milestone 2

✓ Write-ahead logging is implemented correctly.
✓ Torn write recovery works; file is truncated as expected.
✓ Compact is atomic and safe.
✓ Serialize write operations to ensure consistent log order.

Issue to fix:
- You're not fsyncing the directory after rename in Compact. This means a power loss immediately after rename might revert the compaction. Fix: Add a syncDir call.

## Milestone 3

✓ HTTP endpoints work end-to-end.
✓ Graceful shutdown works; server closes cleanly.
✓ Data persists across restarts.
✓ Structured logging with slog is great.

Suggestions:
- Consider adding a metrics endpoint for observability.
- For production, add request timeouts using context.WithTimeout.

## Overall

Excellent work! You've built a real, durable system. Key takeaways:
1. Durability requires careful handling of crashes and atomicity.
2. Concurrency patterns (mutex, channels, context) are essential.
3. Testing with injected dependencies (fake clocks, temp files) is powerful.

Keep practicing these patterns in future projects.
```

---

## References

- **Write-Ahead Logging**: "Designing Data-Intensive Applications" Ch. 4.
- **Graceful Shutdown**: https://pkg.go.dev/net/http#Server.Shutdown
- **slog**: https://pkg.go.dev/log/slog
- **Atomic Renames**: https://www.usenix.org/conference/hotos15/atomicrename-synchronization-free-atomic-visibility-semantics (paywalled, but good to know).
