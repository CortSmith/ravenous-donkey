# Milestone 3: HTTP API & Graceful Shutdown

**Time budget:** ≤ 60 minutes

> **Provided for you:** `store/memory.go`, `store/log.go` and `store/db.go` are complete reference implementations of Milestones 1–2, so this milestone is independent of your earlier code. You implement `store/api.go` and `cmd/kvd/main.go`. Feel free to swap in your own M1/M2 code instead.

## Overview

Build a complete, runnable service: an HTTP API wrapping the durable store from Milestone 2, with structured logging and graceful shutdown. Implement three endpoints (GET /kv/{key}, PUT /kv/{key}, DELETE /kv/{key}), start a server in main, and handle SIGINT/SIGTERM cleanly.

## Specification

### HTTP API

Three endpoints:

**GET /kv/{key}**
- Returns the value associated with key.
- Status 200: `{"value":"..."}` (success).
- Status 404: `{"error":"key not found"}` (key missing or expired).
- Status 400: `{"error":"..."}` (invalid request, e.g., empty key).
- Status 500: `{"error":"database error"}` (internal error).

**PUT /kv/{key}**
- Sets the value associated with key.
- Request body: `{"value":"...","ttl":5000}` (ttl in milliseconds, optional).
- Status 200: `{"key":"...","value":"..."}` (success).
- Status 400: `{"error":"..."}` (invalid request body or empty key).
- Status 500: `{"error":"database error"}` (database error).

**DELETE /kv/{key}**
- Deletes the key.
- Status 200: `{"key":"..."}` (success; key was live).
- Status 404: `{"error":"key not found"}` (key missing or already expired).
- Status 400: `{"error":"..."}` (invalid request, e.g., empty key).
- Status 500: `{"error":"database error"}` (internal error).

### Server

```go
// Server wraps a DB and provides HTTP handlers.
type Server struct {
    db *DB
    lg *slog.Logger
}

// NewServer creates a Server.
func NewServer(db *DB, lg *slog.Logger) *Server

// Register registers the server's routes on the given mux.
func (s *Server) Register(mux *http.ServeMux)
```

### Main Program

A runnable server in `cmd/kvd/main.go`:

```bash
kvd -addr 127.0.0.1:8080 -db /path/to/kvd.log
```

**Behavior:**
1. Parse `-addr` (listen address, default 127.0.0.1:8080) and `-db` (log path, default kvd.log).
2. Open the database at `-db`.
3. Start the janitor to clean up expired keys every minute.
4. Create an HTTP server with the store's routes.
5. Listen and serve on `-addr`.
6. On SIGINT or SIGTERM, gracefully shutdown: stop accepting new requests, wait for in-flight requests, close the database.
7. Log all operations and errors with slog.

`main` itself stays tiny: it sets up signal handling and calls
`run(ctx, args, stdout, stderr) error`, the testable-main pattern from
module 00 (section 6). `cmd/kvd/main_test.go` starts `run` on a random port, talks to it
over real HTTP, cancels its context (exactly what a signal does), and starts it
again on the same log file to prove the data survived.

### Testing

```bash
# from the course root
go test -race ./modules/10-capstone/milestone3/...
go test -race ./modules/10-capstone/milestone3/cmd/kvd/   # just the program
```

Tests verify:
- GET/PUT/DELETE endpoints work end-to-end.
- TTL expiry works (with a fake clock).
- Error responses (404, 400, 500) are correct.
- The program starts, serves, shuts down cleanly on cancellation, and data persists across a restart (`cmd/kvd`).
- Status codes and JSON formatting.

---

## Implementation Guide

### Part 1: Server (store/api.go)

Implement the Server struct and its methods:

1. **NewServer**: Store the DB and logger.

2. **Register**: Use `mux.HandleFunc` to wire up three handlers:
   ```go
   mux.HandleFunc("GET /kv/{key}", s.get)
   mux.HandleFunc("PUT /kv/{key}", s.put)
   mux.HandleFunc("DELETE /kv/{key}", s.delete)
   ```

3. **writeJSON**: A helper that sets Content-Type, status, and encodes JSON. All responses use this.

4. **get**:
   - Extract key via `r.PathValue("key")`.
   - If key is empty, return 400.
   - Call `db.Get(key)`.
   - If ErrNotFound, return 404 with an error message.
   - If error, return 500.
   - If success, return 200 with the value.
   - Log the request with key, status, and any error.

5. **put**:
   - Extract key via `r.PathValue("key")`.
   - If key is empty, return 400.
   - Decode the JSON body into putRequest (error on invalid JSON = 400).
   - Convert TTL from milliseconds to time.Duration (0 if omitted or ≤ 0).
   - Call `db.Set(key, value, ttl)`.
   - If error, return 500.
   - If success, return 200 with key and value.
   - Log the request.

6. **delete**:
   - Extract key via `r.PathValue("key")`.
   - If key is empty, return 400.
   - Call `db.Delete(key)`.
   - If error, return 500.
   - If ok is false (key was absent), return 404.
   - If ok is true, return 200 with the key.
   - Log the request.

**Request/Response types:**
```go
type putRequest struct {
    Value string `json:"value"`
    TTL   int64  `json:"ttl,omitempty"`  // milliseconds
}

type errorResp struct {
    Error string `json:"error"`
}
```

**Logging:**
Use `logger.InfoContext(r.Context(), "event", "key", key, "status", status)` for info-level logs and `logger.ErrorContext` for errors. This gives you request context and structured fields.

### Part 2: Main Program (cmd/kvd/main.go)

`main` is provided. You write `run`:

```go
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error
```

1. **Parse flags** with `flag.NewFlagSet("kvd", flag.ContinueOnError)` and
   `fs.SetOutput(stderr)`: `-addr` (default `127.0.0.1:8080`) and `-db`
   (default `kvd.log`). Return the parse error; don't call `os.Exit` in `run`.

2. **Create the logger** writing to `stdout`:
   `slog.New(slog.NewTextHandler(stdout, nil))`.

3. **Open the database** with `store.Open(dbPath, store.Options{})`, and
   close it on **every** return path. Report the error from `Close`: for a
   log file it can be the only sign that data never reached the disk.
   ```go
   defer func() {
       if cerr := db.Close(); cerr != nil {
           err = errors.Join(err, fmt.Errorf("close database: %w", cerr))
       }
   }()
   ```
   (This needs a named result: `(err error)`.)

4. **Listen** with `net.Listen("tcp", addr)`. Log `"server started"` with
   `"addr", ln.Addr().String()`. With `-addr 127.0.0.1:0` the OS picks the
   port, and the tests read it from this line.

5. **Start the janitor** in a goroutine with a context derived from `ctx`.
   Before closing the database, cancel it and **wait** for the goroutine to
   return (a `done` channel), so nothing touches the DB after `Close`.

6. **Create the server** and register the routes:
   ```go
   mux := http.NewServeMux()
   store.NewServer(db, logger).Register(mux)
   srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
   ```
   `ReadHeaderTimeout` stops a client from holding a connection open forever
   by sending headers one byte at a time (the "Slowloris" attack).

7. **Serve in a goroutine and wait for either outcome**:
   ```go
   serveErr := make(chan error, 1)
   go func() { serveErr <- srv.Serve(ln) }()
   select {
   case err := <-serveErr:
       return fmt.Errorf("serve: %w", err) // failed on its own
   case <-ctx.Done():
   }
   ```
   Waiting only on `<-ctx.Done()` would hang forever if `Serve` failed.

8. **Shut down gracefully**: log `"shutting down"`, call `srv.Shutdown` with
   a 5-second timeout, then check that `<-serveErr` is
   `http.ErrServerClosed` (use `errors.Is`), and log `"shutdown complete"`.

9. **Signals** are `main`'s job: `signal.NotifyContext(ctx, os.Interrupt,
   syscall.SIGTERM)`. `SIGTERM` is what `docker stop`, Kubernetes and systemd
   send. Handling only Ctrl-C (`os.Interrupt`) is a classic production bug.

Try it by hand (or in the container: `docker compose -f modules/10-capstone/compose.yaml --profile app up kvd`,
then `docker compose ... stop kvd` sends the SIGTERM):

```bash
go run ./modules/10-capstone/milestone3/cmd/kvd -db /tmp/kvd.log &
curl -X PUT -d '{"value":"hello"}' localhost:8080/kv/greeting
kill %1                      # SIGTERM: watch the shutdown log lines
go run ./modules/10-capstone/milestone3/cmd/kvd -db /tmp/kvd.log &
curl localhost:8080/kv/greeting   # still there
kill %1
```

---

## Common Mistakes

1. **Not extracting path parameters**: Use `r.PathValue("key")`, not r.URL.Path parsing.

2. **Not handling TTL conversion**: TTL in the request is milliseconds; convert to time.Duration.

3. **Not closing the listener gracefully**: Use `httpServer.Shutdown`, not just close.

4. **Losing `db.Close()` on error paths**: `os.Exit` skips deferred calls, so a `defer db.Close()` in a `main` that later calls `os.Exit(1)` never runs. Keep `os.Exit` out of `run`, and return errors instead.

5. **Not logging with context**: Use `InfoContext`/`ErrorContext` to include request tracing.

6. **Ignoring JSON decoding errors**: Empty request body, malformed JSON, and type mismatches should return 400.

7. **Not handling ErrClosed**: If the database is closed, operations return ErrClosed. Return 500 with a clear message.

8. **Forgetting to start the janitor**: Without it, expired keys never go away. Start it early and pass the shutdown context.

9. **Hardcoding localhost**: Use the `-addr` flag; don't assume 127.0.0.1:8080.

---

## Performance Notes

- **No middleware overhead**: This is a simple server; if you add middleware (timing, tracing, etc.), measure the impact.
- **Graceful shutdown timeout**: 5 seconds is reasonable for small deployments; tune based on your max expected request latency.
- **Janitor interval**: Every minute is fine for small datasets. For millions of keys, consider more frequent cleanup.

---

## Rubric (for instructor review)

When reviewing a student's solution, check:

- ✓ All functions compile and tests pass with `-race`.
- ✓ HTTP endpoints return correct status codes (200, 400, 404, 500).
- ✓ Request/response bodies are valid JSON with expected fields.
- ✓ TTL is converted from milliseconds to time.Duration.
- ✓ GET returns 404 for expired keys.
- ✓ PUT without a TTL stores the key with no expiry.
- ✓ DELETE returns 404 for absent or already-expired keys.
- ✓ Server logs operations with slog (not fmt.Printf).
- ✓ Server gracefully shuts down on SIGINT/SIGTERM.
- ✓ After shutdown, the database is closed and the log is flushed.
- ✓ Data persists across a server restart (durability).
- ✓ `-race` detector finds no issues.

---

## Stretch Goals (Optional)

If you finish early:

1. **Compact on startup**: If the log is large, compact before serving requests.
2. **Metrics endpoint**: Add a GET /metrics endpoint that returns JSON with stats (number of keys, log size, etc.).
3. **Expiration header**: In GET responses, include an optional "Expires" header with the UTC time the key will expire.
4. **Request timeouts**: Use `context.WithTimeout` on database operations so slow queries don't hang the server.
5. **TLS support**: Add flags for cert/key and use `http.Server{TLSConfig:...}`.

---

## Next Steps

When all tests pass, you've completed the capstone! Ask your instructor for feedback on the design and implementation.
