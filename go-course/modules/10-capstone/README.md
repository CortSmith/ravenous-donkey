# Module 10: Capstone — Building a Mini Key-Value Store

Welcome to the capstone! You'll build a durable, concurrent key-value store in three milestones, each building on the last. By the end, you'll have a real system: an in-memory store with TTL expiry, persistence via an append-only log, and an HTTP API.

**Quick facts:**
- **Go 1.24**, stdlib only (as always).
- Three milestones, each ≤ 60 minutes.
- All code passes `-race` (concurrency-safe).
- TTL tests use an injectable clock (no sleeping).
- Later milestones can be started independently; they ship their own starter code.

---

## Goals

By the end of the capstone you will be able to:

- Design a concurrency-safe data structure (`sync.RWMutex`, a janitor goroutine stopped by `context`) and prove it race-free with `-race`.
- Make time testable by injecting a clock, so TTL tests run instantly and deterministically.
- Make data durable with an append-only log: one `Write` per record, `fsync`, replay on start-up, torn-tail recovery, and atomic compaction (write temp file → fsync → rename → fsync directory).
- Explain, with a measured number, what `fsync` costs and what you lose without it.
- Expose a store over HTTP with Go 1.22 `ServeMux` patterns, JSON, and `slog`.
- Write a production-shaped `main`: a testable `run(ctx, args, stdout, stderr) error`, SIGINT/SIGTERM handling, server timeouts, graceful `Shutdown`, and cleanup on every exit path.
- Combine every earlier module (errors, interfaces, testing, I/O, concurrency, HTTP) into one working program.

---

## Module Structure

Each milestone is self-contained:

- **Milestone 1**: concurrency-safe in-memory store with TTL and a janitor goroutine.
- **Milestone 2**: persistence via append-only log, replay on startup, and compaction.
- **Milestone 3**: HTTP API (GET/PUT/DELETE /kv/{key}), slog logging, graceful shutdown, and a runnable server in `cmd/kvd`.

Each milestone directory has:
- A **README.md** with spec, design decisions, and the exact `go test` command.
- **Starter code** in `modules/10-capstone/milestone{1,2,3}/`.
- **Solution** in `solutions/10-capstone/milestone{1,2,3}/`.

---

## Key Concepts by Milestone

### Milestone 1: TTL & Janitor

**What you'll learn:**
- Time-based expiry (lazy + active cleanup).
- Context cancellation for graceful shutdown.
- sync.Mutex / RWMutex patterns.
- Defensive coding: testing with an injectable clock, no sleeps.

**Core ideas:**
- Entries expire at an absolute instant; reads check against "now".
- A janitor goroutine periodically sweeps expired entries so they don't hog memory.
- The janitor stops when the context is canceled, and tests can inject a fake clock.

**Implementation hints:**
- Store entries with an `expiresAt` field (zero = never expires).
- `Set` calculates expiry relative to "now" and stores the absolute time.
- `Get` and other reads treat expired entries as absent (lazy expiry).
- `DeleteExpired` scans the map once per tick; use a `time.Ticker` in `RunJanitor`.

### Milestone 2: Append-Only Log & Replay

**What you'll learn:**
- Durability via write-ahead logging (WAL).
- Handling crashes: torn writes, recovery, file truncation.
- Binary format: JSON Lines for simple, debuggable storage.
- Atomicity via fsync and careful file handling.
- Compaction to shrink the log over time.

**Core ideas:**
- Every write (Set, Delete) is appended to a log before it touches memory.
- On startup, the log is replayed to rebuild state.
- A torn final record (process died mid-write) is detected and discarded.
- Records are stored as JSON lines so corruption is line-granular.
- Compaction rewrites the log to contain one record per live key, removing deleted/overwritten entries.

**Implementation hints:**
- Log records: `{"op":"set","key":"k","value":"v","exp":12345}` and `{"op":"del","key":"k"}`.
- One write() per record ensures crashes tear at most the final record.
- Replay() scans lines, tracking the offset of the last complete line, and truncates if needed.
- Compact() creates a temp file, renames it over the old log, and fsyncs the directory.
- DB serializes writes via a mutex so the log order is always correct.

### Milestone 3: HTTP API & Graceful Shutdown

**What you'll learn:**
- net/http routing (Go 1.22 patterns: GET /path/{id}).
- Request/response encoding (JSON, status codes).
- slog structured logging.
- Graceful shutdown with context cancellation.
- Running a real server in main().

**Core ideas:**
- Three routes: GET /kv/{key} reads, PUT /kv/{key} writes (with optional TTL), DELETE /kv/{key} deletes.
- The server is an http.Handler wrapping the DB.
- Graceful shutdown: catch SIGINT/SIGTERM, cancel the context, wait for active requests.
- Log every operation with context and status for debugging.

**Implementation hints:**
- Use r.PathValue("key") to extract path parameters (Go 1.22+).
- Request body for PUT: `{"value":"...","ttl":5000}` (TTL in milliseconds).
- Response on success: `{"key":"...","value":"..."}` or similar.
- Graceful shutdown: `signal.NotifyContext` + `httpServer.Shutdown` with a timeout.
- Include a main in cmd/kvd that ties everything together.

---

## Concepts: TTL, Janitor, Persistence, HTTP

### TTL and Janitor Patterns

**Lazy vs. Active Expiry:**

In a real system, you often want both:
- **Lazy**: When a reader asks for a key, check if it's expired; if so, return ErrNotFound.
- **Active**: A background task periodically deletes expired entries to reclaim memory.

Together, they guarantee:
- Readers never observe stale data.
- Memory is eventually reclaimed (no unbounded growth).
- Tests can skip the janitor by never calling RunJanitor, or drive it manually with a fake clock.

```go
var ErrNotFound = errors.New("not found")

// Store is a minimal TTL store: just enough around the methods below.
type Store struct {
    mu    sync.RWMutex
    items map[string]entry
    now   func() time.Time
}

func (s *Store) Set(key, value string, ttl time.Duration) {
    s.mu.Lock()
    defer s.mu.Unlock()
    var exp time.Time
    if ttl > 0 {
        exp = s.now().Add(ttl)
    }
    s.items[key] = entry{value: value, expiresAt: exp}
}

// An entry with a TTL.
type entry struct {
    value     string
    expiresAt time.Time // zero = never
}

// Lazy: check on read.
func (e entry) expired(now time.Time) bool {
    return !e.expiresAt.IsZero() && !now.Before(e.expiresAt)
}

// Store.Get treats it as absent if expired.
func (s *Store) Get(key string) (string, error) {
    now := s.now()
    s.mu.RLock()
    e, ok := s.items[key]
    s.mu.RUnlock()
    if !ok || e.expired(now) {
        return "", ErrNotFound
    }
    return e.value, nil
}

// Active: janitor sweep.
func (s *Store) DeleteExpired() int {
    now := s.now()
    s.mu.Lock()
    defer s.mu.Unlock()
    n := 0
    for k, e := range s.items {
        if e.expired(now) {
            delete(s.items, k)
            n++
        }
    }
    return n
}

// Janitor loop: run every interval until ctx is canceled.
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

func main() {
    now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
    s := &Store{items: map[string]entry{}, now: func() time.Time { return now }}
    s.Set("session", "abc", time.Minute)
    s.Set("config", "on", 0) // never expires

    now = now.Add(2 * time.Minute) // time passes
    _, err := s.Get("session")
    fmt.Println(err)                      // not found: lazily expired on read
    fmt.Println(len(s.items))             // 2: but still in memory...
    fmt.Println(s.DeleteExpired())        // 1: ...until the janitor sweeps
    fmt.Println(s.Get("config"))          // on <nil>

    // Drive the janitor with a channel the test controls:
    ctx, cancel := context.WithCancel(context.Background())
    ticks := make(chan time.Time)
    done := make(chan struct{})
    go func() { s.janitor(ctx, ticks); close(done) }()
    ticks <- now // one sweep
    cancel()
    <-done
}
```

**Testable Clocks:**

Never sleep in tests. Instead, inject a fake clock:

```go
// The reference milestone 1 store, so the output means something.
// Point the import at modules/10-capstone/milestone1/store to try yours.
import "gocourse/solutions/10-capstone/milestone1/store"

type fakeClock struct {
    mu  sync.Mutex
    now time.Time
}

func (c *fakeClock) Now() time.Time {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.now = c.now.Add(d)
}

func main() {
    // In tests:
    baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
    clk := &fakeClock{now: baseTime}
    s := store.New(store.WithClock(clk.Now))
    s.Set("key", "val", time.Second)
    fmt.Println(s.Get("key")) // val <nil>
    clk.Advance(time.Second)  // key is now expired
    fmt.Println(s.Get("key")) // store: key not found
    // No sleep needed!
}
```

### Append-Only Logs and Durability

**Write-Ahead Logging (WAL):**

A durable store never updates in-place; it logs first, then applies in memory:

1. **Log append**: Write the mutation to stable storage.
2. **Acknowledge**: Return to caller (write is durable).
3. **Apply in memory**: Update the in-memory state.
4. **Crash recovery**: On restart, replay the log to rebuild state.

Benefits:
- If the process crashes between log and memory, the log has the truth.
- If it crashes during the memory update, the next replay will redo it (idempotent).

**JSON Lines Format:**

Store one record per line, as JSON:

```
{"op":"set","key":"user:1","value":"alice","exp":1735787045000000000}
{"op":"set","key":"user:2","value":"bob","exp":0}
{"op":"del","key":"user:1"}
{"op":"set","key":"user:1","value":"alice2","exp":1735787045000000000}
```

Why JSON Lines?
- Line-by-line parsing is simple and streaming-friendly.
- Each '\n' is a frame boundary; a torn write stops at the last newline.
- Corruption is isolated to one line, not the entire file.
- Human-readable (great for debugging).

**Handling Torn Writes:**

If the process dies mid-Append, the last line may be incomplete:

```
{"op":"set","key":"k","value":"v","exp":123
<-- process dies here; '\n' never written -->
```

On Replay, we detect this:
- Use bufio.ReadBytes('\n') to read lines.
- If EOF is hit and there's unread data, that's a torn record.
- Truncate the file to discard the torn record.
- Continue; the torn record was never acknowledged, so losing it is safe.

```go
// A log whose last record was torn by a crash: no final '\n'.
f, _ := os.CreateTemp("", "torn-*.log")
defer os.Remove(f.Name())
f.WriteString("{\"op\":\"set\",\"key\":\"a\"}\n{\"op\":\"set\",\"key\":\"b\"}\n{\"op\":\"set\",\"key\":\"k\",\"val")
f.Seek(0, io.SeekStart)

br := bufio.NewReader(f)
var lastGoodOffset int64
for {
    line, err := br.ReadBytes('\n')
    if err == io.EOF {
        if len(line) > 0 { // torn tail
            fmt.Printf("torn record: %q\n", line)
            f.Truncate(lastGoodOffset)
        }
        break
    }
    fmt.Printf("replay: %s", line) // process line...
    lastGoodOffset += int64(len(line))
}
info, _ := f.Stat()
fmt.Println("file size now", info.Size(), "=", lastGoodOffset)
```

**Compaction:**

Over time, the log grows with deletes and rewrites. Compaction shrinks it:

1. Snapshot the in-memory state (call Items()).
2. Write one "set" record per live key to a temp file.
3. fsync the temp file (data durable).
4. Atomic rename over the old log.
5. fsync the directory (rename durable).
6. Reopen the log with O_APPEND.

```go
type Record struct {
    Op  string `json:"op"`
    Key string `json:"key"`
}

type Log struct {
    path string
    f    *os.File
}

func (l *Log) Compact(records []Record) error {
    dir := filepath.Dir(l.path)

    // Write to temp file (same directory, so the rename stays on one filesystem)
    tmp, err := os.CreateTemp(dir, "compact-*")
    if err != nil {
        return err
    }
    defer os.Remove(tmp.Name()) // no-op after a successful rename
    enc := json.NewEncoder(tmp)
    for _, r := range records {
        if err := enc.Encode(r); err != nil {
            tmp.Close()
            return err
        }
    }
    if err := tmp.Sync(); err != nil { // durable on disk
        tmp.Close()
        return err
    }
    if err := tmp.Close(); err != nil {
        return err
    }

    if err := os.Rename(tmp.Name(), l.path); err != nil { // atomic
        return err
    }
    if err := syncDir(dir); err != nil { // make rename durable
        return err
    }

    // Reopen with O_APPEND
    f, err := os.OpenFile(l.path, os.O_RDWR|os.O_APPEND, 0o644)
    if err != nil {
        return err
    }
    l.f.Close()
    l.f = f
    return nil
}

func syncDir(dir string) error {
    d, err := os.Open(dir)
    if err != nil {
        return err
    }
    defer d.Close()
    return d.Sync()
}

func main() {
    dir, _ := os.MkdirTemp("", "compact")
    defer os.RemoveAll(dir)
    path := filepath.Join(dir, "kv.log")
    os.WriteFile(path, []byte("{\"op\":\"set\",\"key\":\"a\"}\n{\"op\":\"del\",\"key\":\"a\"}\n{\"op\":\"set\",\"key\":\"b\"}\n"), 0o644)
    f, _ := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0o644)
    l := &Log{path: path, f: f}

    fmt.Println(l.Compact([]Record{{Op: "set", Key: "b"}})) // <nil>
    data, _ := os.ReadFile(path)
    fmt.Print(string(data)) // only the live key is left
}
```

### HTTP API and Graceful Shutdown

**Routing (Go 1.22):**

ServeMux now supports method and path parameters:

```go
mux := http.NewServeMux()

// GET /kv/{key}: read
mux.HandleFunc("GET /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	fmt.Fprintln(w, "read", key) // look the key up in the db
})

// PUT /kv/{key}: write
mux.HandleFunc("PUT /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
	// decode JSON body, then call db.Set
	fmt.Fprintln(w, "write", r.PathValue("key"))
})

// DELETE /kv/{key}: delete
mux.HandleFunc("DELETE /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
	// call db.Delete
	fmt.Fprintln(w, "delete", r.PathValue("key"))
})

for _, m := range []string{"GET", "PUT", "DELETE", "PATCH"} {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(m, "/kv/user:1", nil))
	fmt.Print(m, " ", rec.Code, " ", rec.Body.String()) // PATCH gets 405
}
```

**Request/Response:**

```go
// PUT request body
type putRequest struct {
    Value string `json:"value"`
    TTL   int64  `json:"ttl,omitempty"` // milliseconds, optional
}

// Success response
type putResp struct {
    Key   string `json:"key"`
    Value string `json:"value"`
}

// Error response
type errorResp struct {
    Error string `json:"error"`
}

func put(w http.ResponseWriter, r *http.Request) {
    var req putRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(errorResp{Error: "invalid JSON"})
        return
    }
    // Set Content-Type and status, then Encode
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(putResp{Key: r.PathValue("key"), Value: req.Value})
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("PUT /kv/{key}", put)
    for _, body := range []string{`{"value": "alice", "ttl": 5000}`, `{oops`} {
        rec := httptest.NewRecorder()
        mux.ServeHTTP(rec, httptest.NewRequest("PUT", "/kv/user:1", strings.NewReader(body)))
        fmt.Print(rec.Code, " ", rec.Body.String())
    }
}
```

This writes the status before encoding, which is fine for these small,
always-encodable structs. For arbitrary values, use module 08's
encode-into-a-buffer-first pattern, so an encoding error can still become a
clean 500.

**Graceful Shutdown:**

Turn signals into context cancellation in `main`, and keep everything else
in a testable `run`:

```go
func main() {
    // SIGTERM is what docker stop, Kubernetes and systemd send, not just Ctrl-C.
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    // Demo only: pretend a SIGTERM arrives after 200ms, so this example ends.
    ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
    defer cancel()
    if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
        fmt.Fprintln(os.Stderr, "kvd:", err)
        stop()
        os.Exit(1) // only here: os.Exit skips deferred calls
    }
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
    ln, err := net.Listen("tcp", "127.0.0.1:0") // any free port
    if err != nil {
        return err
    }
    srv := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: 5 * time.Second}
    fmt.Fprintln(stdout, "listening on", ln.Addr())

    // After starting the server:
    serveErr := make(chan error, 1)
    go func() { serveErr <- srv.Serve(ln) }()

    select {
    case err := <-serveErr: // Serve failed by itself: don't wait for a signal forever
        return fmt.Errorf("serve: %w", err)
    case <-ctx.Done(): // signal (or a test) asked us to stop
    }

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := srv.Shutdown(shutdownCtx); err != nil { // stop accepting, drain in-flight requests
        return fmt.Errorf("shutdown: %w", err)
    }
    fmt.Fprintln(stdout, "shut down cleanly")
    return nil
}
```

Note that `Shutdown` gets a **fresh** context: `ctx` is already canceled at
this point, and passing it would abort the drain immediately. Milestone 3's
README walks through the full `run`.

**Structured Logging:**

Use slog (Go 1.21+) for structured output:

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

logger.Info("server started", "addr", "127.0.0.1:8080")

r := httptest.NewRequest("GET", "/kv/user:1", nil)
key, err := "user:1", errors.New("disk read failed")
logger.ErrorContext(r.Context(), "get failed", "key", key, "error", err)
```

---

## Idioms & Best Practices

### Channels and Goroutine Lifecycle

- **Always stop a Ticker**: `defer t.Stop()` when creating a new Ticker.
- **Context cancellation**: Pass `ctx` to goroutines and check `ctx.Done()` in loops.
- **Goroutine leaks**: If a goroutine never returns, it's a leak. Test with `runtime.NumGoroutine()` or timeout-based checks.

### Error Handling

- **Sentinel errors**: Define package-level errors like `var ErrNotFound = errors.New("...")` for specific conditions.
- **Wrapping**: Use `fmt.Errorf("%w", err)` to chain errors.
- **Checking**: Use `errors.Is(err, ErrNotFound)` to check sentinel errors.

### Concurrency

- **RWMutex over Mutex**: Use RWMutex when you have many readers and few writers. Release the lock as soon as possible (don't hold it during expensive operations like sorting).
- **Serializing writes**: Use a mutex to ensure writes (and reads that depend on consistency) are ordered.
- **Race detector**: Always test with `-race` on concurrency-heavy code.

### Slices and Maps

- **nil vs. empty**: Return `nil` or an empty slice; both are valid. Don't mix them inconsistently.
- **Preallocation**: If you know the size upfront, preallocate to avoid repeated allocations.
- **Sorting**: Use `slices.Sort` (Go 1.21+) for simple types, `slices.SortFunc` for custom orders.

---

## Common Pitfalls

1. **Not stopping Tickers**: before Go 1.23 an unstopped `time.Ticker` was never garbage-collected (a leak). Since 1.23 it is collected once unreferenced, but `defer t.Stop()` still documents exactly when the ticker's life ends. Write it anyway.
2. **Holding locks too long**: Locks block readers. Release as soon as possible; sort/encode outside the lock if possible.
3. **Ignoring torn writes**: If you don't truncate the log after a torn tail, subsequent Replays may read garbage.
4. **Forgetting fsync**: If you don't fsync after writes, a power loss can lose data. fsync is slow but essential for durability.
5. **Not fsyncing the directory**: After renaming a file, fsync the directory so the rename itself is durable (not just the file contents).
6. **Calling `time.Now()` directly**: one stray `time.Now()` in `Get` or `DeleteExpired` bypasses the injected clock, and the TTL tests become flaky or wrong. Always call `s.now()`. (Deleting from a map *during* `range` over it is fine in Go; the spec allows it.)
7. **Race conditions on maps**: Maps are not thread-safe. Always guard with a mutex, even if you think a single write is atomic.

---

## Exercises

The capstone has no separate exercises: the three milestones are the
exercises. Each has its own README with the full spec and test command, and
ships complete working code for the earlier milestones under "Provided for
you", so you can start any of them independently.

| Milestone | Spec | You implement | Test command |
|---|---|---|---|
| 1. In-memory store with TTL | `milestone1/README.md` | `store/store.go` | `go test -race ./modules/10-capstone/milestone1/...` |
| 2. Append-only log & replay | `milestone2/README.md` | `store/log.go`, `store/db.go` | `go test -race ./modules/10-capstone/milestone2/...` |
| 3. HTTP API & graceful shutdown | `milestone3/README.md` | `store/api.go`, `run` in `cmd/kvd/main.go` | `go test -race ./modules/10-capstone/milestone3/...` |

## Project

The finished milestone 3 *is* the project: a runnable `kvd` server. When its
tests pass, run it for real. Start it, `curl` a few keys, stop it with
`kill` (SIGTERM), start it again, and check the keys are still there. The
commands are at the end of `milestone3/README.md`. Stretch goals (group
commit, the sharded store, metrics) are listed in each milestone's README.

---

## Submitting for Review

When you complete all three milestones and tests pass with `-race`:

```bash
go test ./modules/10-capstone/milestone1/... -race
go test ./modules/10-capstone/milestone2/... -race
go test ./modules/10-capstone/milestone3/... -race
```

Ask your instructor (Claude) to review `modules/10-capstone/`. They'll check:
- Code compiles and passes tests (including `-race`).
- TTL tests use a clock, not sleeps.
- Serialization is correct (writes are ordered, compaction is atomic).
- HTTP API works end-to-end.
- Graceful shutdown on signal.

---

## Further Reading

- **Effective Go**: https://go.dev/doc/effective_go
- **Go Code Review Comments**: https://go.dev/wiki/CodeReviewComments
- **net/http `Server.Shutdown`**: https://pkg.go.dev/net/http#Server.Shutdown
- **`os/signal.NotifyContext`**: https://pkg.go.dev/os/signal#NotifyContext
- **Concurrency patterns**: https://go.dev/blog/pipelines
- **net/http**: https://pkg.go.dev/net/http
- **log/slog**: https://pkg.go.dev/log/slog
- **Context package**: https://pkg.go.dev/context
- **Durability**: "Designing Data-Intensive Applications" by Martin Kleppmann (external reading).
