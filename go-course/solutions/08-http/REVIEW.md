# Review Guide for Module 08: HTTP

## Exercise 1: health

### Readable Solution
The `Handler` function creates an http.Handler that:
1. Initializes a Response with status "ok"
2. Iterates over each check with a fresh per-check timeout using `context.WithTimeout`
3. Records "ok" or the error message in the Checks map
4. Returns HTTP 200 if all pass, HTTP 503 if any fail

The key design insight is using `context.WithTimeout` for **each check separately**. This means if one check times out, it doesn't steal time from the others. A naive implementation might use `context.WithDeadline` once at the top and share that context across all checks—that would mean if the first check uses 8 of 10 seconds, the second only gets 2. By creating a fresh timeout for each check, you give each one its full allotment.

**Example structure:**
```go
import "gocourse/solutions/08-http/exercises/health"

type (
	Check    = health.Check
	Response = health.Response
)

func Handler(timeout time.Duration, checks map[string]Check) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        resp := Response{
            Status: "ok",
            Checks: make(map[string]string, len(checks)),
        }

        for name, check := range checks {
            ctx, cancel := context.WithTimeout(r.Context(), timeout)
            err := check(ctx)
            cancel() // Release the timer immediately

            if err != nil {
                resp.Status = "fail"
                resp.Checks[name] = err.Error()
            } else {
                resp.Checks[name] = "ok"
            }
        }

        status := http.StatusOK
        if resp.Status == "fail" {
            status = http.StatusServiceUnavailable
        }

        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Cache-Control", "no-store")
        w.WriteHeader(status)
        json.NewEncoder(w).Encode(resp)
    })
}

func main() {
	checks := map[string]Check{
		"db":    func(ctx context.Context) error { return nil },
		"cache": func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }, // hangs until the timeout
	}
	rec := httptest.NewRecorder()
	Handler(50*time.Millisecond, checks).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	fmt.Print(rec.Code, " ", rec.Body.String()) // 503 {"status":"fail","checks":{"cache":"context deadline exceeded","db":"ok"}}
}
```

### Optimized Solution
`HandlerParallel` (in `fast.go`) runs every check in its own goroutine, so
the endpoint's latency is that of the **slowest** check instead of the **sum**
of all of them. Each goroutine writes only its own slot of a pre-sized
`[]error`, so no mutex is needed, and the result map is built after
`wg.Wait()`. The check names are copied into a slice once, at construction
time.

**Measured (Go 1.24.2 linux/amd64, `-count=5`, medians), three checks that
each sleep 1 ms:**
- `Handler` (sequential): ~3.29 ms/op, 44 allocs/op
- `HandlerParallel`: ~1.17 ms/op, 49 allocs/op

That is about 2.8x lower latency, at the cost of a goroutine per check (5 more
allocations). The CPU cost is irrelevant here; the win is entirely in
wall-clock latency, which is what a load balancer probing `/healthz`
experiences.

### Trade-off
- **Sequential** is simpler, deterministic, and easy to test. Prefer it when
  checks are fast (in-process) or few.
- **Parallel** is worth it as soon as checks do I/O (DB ping, upstream HTTP).
  The price is goroutine reasoning: each goroutine must write only its own
  slot (or use a mutex), and the handler must not return before `wg.Wait()`.
  Run the tests with `-race`.
- In both versions, the per-check `context.WithTimeout` (not one shared
  deadline) is what keeps one slow dependency from eating the others' budget.

### Review Checklist
- [ ] Handler returns an http.Handler (implements ServeHTTP), not a plain function
- [ ] Response status is exactly "ok" or "fail" (not "healthy", "unhealthy", "pass", "success")
- [ ] Checks map is always initialized (not nil), even with zero checks
- [ ] Uses `context.WithTimeout`, not `WithDeadline` or `WithCancel`
- [ ] Each check runs in its own independent context (creates a new timeout for each iteration)
- [ ] `cancel()` is called after every check (deferred or explicit after the check call)
- [ ] HTTP status 200 is used for all-pass, 503 for any failure (not 500)
- [ ] Cache-Control header is set to "no-store" (health responses should never be cached)
- [ ] Content-Type is set to "application/json"
- [ ] Response is JSON-encoded with json.NewEncoder or json.Marshal (not plain text or skipped)
- [ ] Handler respects the request context (derives child contexts from r.Context(), not context.Background())
- [ ] Concurrent checks don't race (each check has its own context; no shared mutable state in the map)
- [ ] Tests use httptest.NewRecorder (no real HTTP server)
- [ ] Tests use httptest.NewRequestWithContext to set timeouts and contexts
- [ ] Tests verify both "ok" and "fail" responses
- [ ] Tests verify that check timeouts are independent (one timeout doesn't affect the next check)

## Exercise 2: jsonio

### Readable Solution
Three functions to keep JSON handling consistent across all handlers:

1. **WriteJSON(w http.ResponseWriter, status int, v any)**: Encodes into a buffer, then writes status and body atomically. If encoding fails, the handler sends HTTP 500 before any partial body.
   - Allocates a bytes.Buffer
   - Uses json.NewEncoder to encode v
   - On encoding error: sends 500 via http.Error
   - On success: sets Content-Type, writes status, then body

2. **WriteError(w, status, msg)**: Helper to send error responses as JSON. Always succeeds because map[string]string always encodes, so write errors are silently ignored (client is gone).

3. **DecodeJSON(w, r, dst)**: Validates the request and decodes into dst, returning *RequestError on client errors, or a server error (for unexpected failures). Does:
   - Check Content-Type is application/json (using mime.ParseMediaType to handle charset)
   - Cap body size with http.MaxBytesReader
   - Call DisallowUnknownFields() on the decoder
   - Detect trailing JSON values (reject multiple documents)
   - Distinguish client errors (400/413/415) from server errors (500)

**Key insight for WriteJSON**: The atomic write is critical. If you write the status code or headers first, then fail to encode, the client has already received a 201 or 200. Now they see a partial body or connection close, which is worse than a clean 500. By buffering, you guarantee either a complete response or a clean error.

**Key insight for DecodeJSON**: Content-Type validation prevents clients from sending binary data and claiming it's JSON. DisallowUnknownFields() catches API versioning issues (e.g., client sends "user_ids" but server expects "user_id"). Trailing-value detection prevents a buggy client from sending two JSON documents in one request.

### Optimized Solution
`WriteJSONFast` (in `fast.go`) makes two changes:
- It takes the encode buffer from a `sync.Pool` instead of allocating a new
  `bytes.Buffer` per response. Buffers that grew past 64 KiB are not returned to
  the pool, so one huge response cannot pin memory forever.
- It assigns `w.Header()["Content-Type"] = jsonContentType`, a shared,
  pre-built `[]string`, instead of `Header().Set`, which allocates a new
  one-element slice every call. This is only valid because the key is already
  in canonical form.

**Measured (Go 1.24.2 linux/amd64, `-count=5`, medians):**
- `WriteJSON`: ~480 ns/op, 184 B/op, 4 allocs/op
- `WriteJSONFast`: ~309 ns/op, 24 B/op, 1 alloc/op
- About 1.5x faster, with 3 fewer allocations per response.

Caveats worth discussing with the student: the shared header slice is a
latent bug if any later code does `w.Header()["Content-Type"][0] = ...` (it
would rewrite the value for every response in the process). `Header().Add`
is safe, because `append` on a full slice copies. A `sync.Pool` also adds
behaviour that is hard to test (buffer reuse between requests). Neither is
worth it unless a profile shows `WriteJSON` on the hot path.

### Trade-off
- **Use readable WriteJSON** for most code. The buffer overhead is worth the clarity.
- **Use an optimized version** only if profiling shows this on the critical path (e.g., thousands of requests per second on single-threaded handler).
- The error translation in `decodeError()` must be explicit; consolidating cases would lose specificity (e.g., distinguish EOF "empty body" from syntax "malformed JSON").

### Review Checklist
- [ ] WriteJSON encodes to a bytes.Buffer first, before any write
- [ ] WriteJSON returns an error only on encoding failure
- [ ] WriteJSON sets Content-Type before WriteHeader
- [ ] WriteJSON never calls WriteHeader on error; lets http.Error do it
- [ ] WriteError creates a map[string]string{...} and ignores write errors
- [ ] DecodeJSON validates Content-Type with mime.ParseMediaType (handles charset, case-insensitivity)
- [ ] DecodeJSON returns 415 Unsupported Media Type for wrong Content-Type
- [ ] DecodeJSON caps body size with http.MaxBytesReader (returns 413 Payload Too Large)
- [ ] DecodeJSON calls dec.DisallowUnknownFields()
- [ ] DecodeJSON returns 400 Bad Request for unknown fields
- [ ] DecodeJSON checks for a second JSON value (reads and expects io.EOF)
- [ ] DecodeJSON returns 400 for multiple documents (trailing values)
- [ ] DecodeJSON handles io.EOF (empty body) separately from other decode errors
- [ ] *RequestError type has both Status (int) and Msg (string) fields
- [ ] *RequestError implements the error interface (Error() string method)
- [ ] All client errors (Content-Type, body size, JSON format, unknown fields) are 4xx
- [ ] All server errors (unexpected Marshal failures) are 5xx
- [ ] Tests cover all error cases: missing header, wrong type, syntax, unknown fields, too large, trailing values
- [ ] Tests use httptest.NewRecorder (no real HTTP server)

## Exercise 3: middleware

### Readable Solution
A classic middleware pattern with four components:

1. **Chain(h, mws...)**: Applies middleware from the end of the slice backwards so the **first** middleware in the slice is the **outermost** (runs first on the request, last on the response). 
   - Example: Chain(h, Logging, Auth) wraps as Auth(Logging(h)), so request flow is: Logging → Auth → h → (response flows backward)

2. **RequestID(gen func() string)**: Middleware factory that assigns each request a unique ID.
   - Reads X-Request-ID header; if missing, calls gen() to create one
   - Bounds the header to 64 bytes (prevents logging 10MB request IDs)
   - Stores ID in request context with an unexported key type (prevents context pollution)
   - Echoes ID in response header for client tracing

3. **Logging(logger)**: Middleware that logs every request and response.
   - Wraps the handler with a statusWriter to capture status and bytes written
   - Logs **after** the handler completes (deferred) with: method, path, status, bytes, duration
   - Includes request_id in the log entry (if RequestID middleware is in the chain)
   - Uses logger.InfoContext (not logger.Info) to carry context attributes

4. **Recover(logger)**: Middleware that catches panics in handlers.
   - Uses defer + recover() to catch panic values
   - Logs panic details at Error level
   - Sends HTTP 500 only if no response has started (checks statusWriter.wroteHeader)
   - Re-panics http.ErrAbortHandler (the standard library's official abort signal for framework use)
   - Does nothing if no panic occurs

5. **statusWriter**: Internal wrapper around http.ResponseWriter.
   - Tracks the status code (recorded on first WriteHeader call)
   - Tracks bytes written (sum of all Write calls)
   - Implements Unwrap() to satisfy http.ResponseController expectations (allows middleware to unwrap and access the real writer)
   - Returns implicit status 200 if Write is called without WriteHeader

**Key insight**: Middleware order matters. Logging should wrap Auth so you log all requests (including rejected ones). Error recovery should be outermost to catch panics from any inner middleware.

### Optimized Solution
`LoggingFast` (in `fast.go`) makes three changes:
- It checks `logger.Enabled(ctx, slog.LevelInfo)` first and, if logging is
  off, calls the next handler directly: no clock reads, no `statusWriter`.
- It logs with `logger.LogAttrs` and typed attributes (`slog.Int`,
  `slog.Duration`), so values are not boxed into `any` as they are with the
  `"key", value` form.
- It collects the attributes in a fixed `[6]slog.Attr` array on the stack.

**Measured (Go 1.24.2 linux/amd64, `-count=5`, medians):**

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `Logging`, Info enabled | ~930 | 82 | 4 |
| `LoggingFast`, Info enabled | ~800 | 50 | 2 |
| `Logging`, Info disabled | ~170 | 82 | 4 |
| `LoggingFast`, Info disabled | ~18 | 2 | 1 |

The cost of middleware composition is dominated by slog formatting, not the wrapper itself.

### Trade-off
- **Use readable implementation** (current) for clarity; the statusWriter wrapper is necessary and has minimal overhead.
- **Optimize only if** profiling shows middleware on critical path. Most applications spend >99% time in handlers, not middleware.
- **Never optimize** by removing panic recovery; a single panic in production without recovery brings down the entire server.

### Review Checklist
- [ ] Chain applies middleware in the correct order (first in slice is outermost)
- [ ] Chain(h) with empty middleware returns h unchanged (identity function for middleware)
- [ ] RequestID uses unexported key type (prevents namespace collisions in context)
- [ ] RequestID generates a new ID via gen() if header is missing
- [ ] RequestID reads from X-Request-ID header and uses it as-is (if present)
- [ ] RequestID bounds header length (maxRequestIDLen = 64; longer headers are replaced with generated ID)
- [ ] RequestID stores ID in context and sets it in response X-Request-ID header
- [ ] Logging wraps handler in statusWriter to capture status and bytes
- [ ] Logging logs **after** handler returns (uses defer)
- [ ] Logging records: method, path, status, bytes written, duration
- [ ] Logging includes request_id in log fields (if context has it)
- [ ] Logging uses logger.InfoContext (carries context attributes like request ID)
- [ ] Logging with disabled slog level is cheap (~18 ns/op, no allocation)
- [ ] Recover catches panic values with recover()
- [ ] Recover logs panic details with logger.ErrorContext
- [ ] Recover re-panics http.ErrAbortHandler (preserves framework abort signal)
- [ ] Recover sends HTTP 500 only if statusWriter.wroteHeader is false
- [ ] Recover sends 500 via http.Error or WriteHeader+Write (not panic)
- [ ] statusWriter implements Write, WriteHeader, Header, and Unwrap methods
- [ ] statusWriter.statusCode() returns 200 for implicit status (when no WriteHeader called yet)
- [ ] statusWriter tracks bytes written correctly across multiple Write calls
- [ ] statusWriter.Unwrap() returns the underlying http.ResponseWriter
- [ ] Flusher/Pusher/Hijacker interfaces are delegated (if present in original writer)
- [ ] Tests cover: no panic, panic before writing, panic after writing, ErrAbortHandler re-panic
- [ ] Tests verify middleware order with a marker or mock (first in slice is outermost)
- [ ] Tests use httptest.NewRecorder and httptest.NewRequestWithContext
- [ ] Tests verify that RequestID persists across context boundaries

## Exercise 4: routes

### Readable Solution
Uses Go 1.22 ServeMux to route based on method and path:

1. **Routes(s *Store)**: Factory function that creates an http.ServeMux, registers all routes, and returns it as http.Handler.
   - No global state; routes are scoped to the returned handler
   - Allows testing multiple stores/configurations independently

2. **Handlers**: Each handler reads path parameters with r.PathValue("name"), validates input, and calls the store.
   - GET /{$} — root path (exact match; /{$} prevents matching "" as a prefix)
   - GET /notes — list all notes (returns 200 with empty array for no notes, not 404)
   - POST /notes — create note; expects JSON {text: "..."}, returns 201 with Location header
   - GET /notes/latest — most recent note (more specific than GET /notes/{id}, so registered first)
   - GET /notes/{id} — fetch one note; {id} captures a single path segment
   - DELETE /notes/{id} — delete note, returns 204 (no content)
   - GET /files/{path...} — serve files; {path...} captures rest of path including slashes

3. **parseID**: Helper that converts r.PathValue("id") to int, validates it's positive, and returns (id, ok).
   - Returns (0, false) for non-numeric input (client error)
   - Returns (0, false) for ≤0 (invalid IDs)
   - Callers use: `id, ok := parseID(r); if !ok { http.Error(...); return }`

4. **writeJSON/writeError**: Helpers to set Content-Type and encode JSON.
   - writeJSON encodes resp to w; returns error on failure
   - writeError sends {"error": msg} as JSON

**Key insight for Go 1.22+ patterns:**
- `GET /items/{id}` matches "GET /items/123" and makes r.PathValue("id") return "123"
- `GET /items/latest` is **more specific** than `GET /items/{id}` so ServeMux tries it first (no manual ordering needed)
- `GET /{$}` matches only "/" (exact root); without {$}, "/" also matches all unregistered paths as a catch-all
- `GET /files/{path...}` captures "a/b/c.txt" from "GET /files/a/b/c.txt" → r.PathValue("path") == "a/b/c.txt"
- Pattern matching is case-sensitive and stops at the first match (most specific wins)

### Optimized Solution
Current solution is already clean. Minor variations:
- Use a generic handler factory if repeating validation (overkill here)
- Type switch on error type for detailed messages (adds complexity; simple message is fine)

The explicit handler methods and parseID helper keep the code maintainable without premature optimization.

### Trade-off
Readable and optimized are the same. The handler structure is the Go idiomatic way: explicit methods, simple helpers, no reflection.

### Review Checklist
- [ ] Routes creates an http.ServeMux
- [ ] Routes returns the mux as http.Handler (or *http.ServeMux, which satisfies the interface)
- [ ] Registers GET /{$} for exact root (not GET /)
- [ ] Registers GET /notes (list) before any other /notes/{...} route (handled by ServeMux specificity)
- [ ] Registers GET /notes/latest before GET /notes/{id} (more specific)
- [ ] Registers POST /notes for creation (separate from GET /notes for listing)
- [ ] POST /notes returns 201 Created with Location header pointing to the new note
- [ ] GET /notes lists all notes; returns [] (empty array) if none exist (not 404 or null)
- [ ] GET /notes/{id} returns 200 with the note
- [ ] DELETE /notes/{id} returns 204 No Content (no body)
- [ ] DELETE and GET /notes/{id} return 400 Bad Request for invalid {id}
- [ ] parseID validates {id} is numeric and positive; returns (0, false) on error
- [ ] All error responses are JSON {"error": "message"}
- [ ] All success responses have Content-Type: application/json
- [ ] Handlers use r.PathValue("id") to extract path parameters
- [ ] GET /files/{path...} correctly captures multi-segment paths (a/b/c.txt)
- [ ] Root handler returns 200 (or appropriate status); doesn't panic on missing path
- [ ] Handlers call store methods (Get, Create, Delete, List, Latest) as designed
- [ ] Handlers respect request context (pass r.Context() to store methods)
- [ ] Tests verify routing with different methods (GET, POST, DELETE, PUT, PATCH)
- [ ] Tests verify method-not-allowed (405) for unsupported methods
- [ ] Tests use httptest.NewRecorder for response inspection
- [ ] Tests use httptest.NewServer for full integration testing
- [ ] Tests verify path parameter extraction for various inputs (123, abc, 0, negative)
- [ ] Tests verify Location header on 201 responses
- [ ] Tests verify wildcard path capture for {path...}

## Exercise 5: client

### Readable Solution
A well-behaved HTTP client with three methods:

1. **New(baseURL, timeout)**: Factory that creates a Client.
   - Stores baseURL with trailing "/" removed (so callers can write "/items/" and it becomes correct)
   - Creates an http.Client with the given timeout
   - Timeout applies to the entire request+response (not just connection establishment)
   - Timeout is critical: prevents goroutine leaks if server never responds

2. **GetItem(ctx, id)**: Fetches GET {baseURL}/items/{id}.
   - Uses http.NewRequestWithContext(ctx, ...) to respect caller's timeout/cancellation
   - Sets Accept: application/json to prefer JSON response
   - Calls helper do() to handle response
   - Returns ErrNotFound (typed error) for 404; callers can use errors.Is(err, ErrNotFound)

3. **CreateItem(ctx, it)**: Posts a new item as JSON.
   - Marshals the item to JSON first (catches errors before making request)
   - Constructs POST request with Content-Type: application/json
   - Expects 201 Created; any other 2xx is treated as error

4. **do(ctx, method, path, body)**: Internal helper that:
   - Constructs the full URL (baseURL + path)
   - Creates request with http.NewRequestWithContext
   - Calls c.http.Do(req)
   - **Always drains the body** even on error (crucial for connection reuse)
     ```go
     srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
     	fmt.Fprint(w, "a body nobody reads")
     }))
     defer srv.Close()

     resp, err := http.Get(srv.URL)
     if err != nil {
     	log.Fatal(err)
     }
     defer resp.Body.Close()
     defer io.Copy(io.Discard, resp.Body) // drain to allow reuse (deferred calls run last-in, first-out: drain, then close)
     fmt.Println(resp.StatusCode)
     ```
   - Checks StatusCode explicitly (404 → ErrNotFound; others → StatusError)
   - Reads body on error for error message (only if small enough)
   - Decodes JSON on success
   - Wraps all errors with context (operation: "get item" or "create item")

**Key insight for draining**: If you don't drain the response body, the TCP connection stays "blocked" waiting for you to read it. The HTTP connection is not returned to the pool, so future requests might create new connections. On high-traffic systems, this leaks sockets and eventually exhausts the client's connection pool.

**Key insight for context passing**: The caller's context carries cancellation (e.g., request timeout in a handler). By passing it to NewRequestWithContext, you respect that deadline. If the handler times out, Do() will be canceled and return immediately.

**Key insight for typed errors**: ErrNotFound is a sentinel value, so callers can use:
```go
import "gocourse/solutions/08-http/exercises/client"

var ErrNotFound = client.ErrNotFound

func main() {
	api := httptest.NewServer(http.NotFoundHandler()) // an API that has no items
	defer api.Close()
	c := client.New(api.URL, time.Second)

	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx, id := r.Context(), r.PathValue("id")
		item, err := c.GetItem(ctx, id)
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "item not found", http.StatusNotFound)
			return
		}
		fmt.Fprintln(w, item)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", handler)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/items/7", nil))
	fmt.Print(rec.Code, " ", rec.Body.String()) // 404 item not found
}
```
This is better than checking error messages (fragile) or error types (verbose).

### Optimized Solution
Could reduce allocations by:
- Reusing a bytes.Buffer pool for request bodies (minor; JSON Marshal already allocates)
- Pre-allocating URL strings (minor; string concatenation is cheap)
- Caching requests (not safe; http.Request is mutable)

The standard library's http.Client already uses connection pooling and reuses buffers internally. Further optimization is not worth the complexity.

### Trade-off
Readable and optimized are the same. The error handling (drain, check status, wrap) and context passing are non-negotiable for correct, production-quality clients.

### Review Checklist
- [ ] New returns *Client (or Client if no pointers needed)
- [ ] New trims trailing "/" from baseURL (so "/items/" becomes "/items")
- [ ] New stores an http.Client with the given timeout
- [ ] GetItem uses http.NewRequestWithContext (not context.Background)
- [ ] GetItem constructs URL as baseURL + "/items/" + id
- [ ] GetItem sets Accept: application/json header
- [ ] GetItem calls do() with "GET" method
- [ ] GetItem returns ErrNotFound (typed error) for 404
- [ ] GetItem wraps other errors with context ("get item: ...")
- [ ] CreateItem marshals the item to JSON before making request (validates early)
- [ ] CreateItem constructs POST request with Content-Type: application/json
- [ ] CreateItem expects 201 Created; 200 is an error
- [ ] do() creates full URL correctly (baseURL + path)
- [ ] do() always defers resp.Body.Close()
- [ ] do() always defers io.Copy(io.Discard, resp.Body) (drains for reuse)
- [ ] do() checks StatusCode explicitly (not generic 2xx check)
- [ ] do() reads response body **after** draining (order matters)
- [ ] do() reads truncated body on error (e.g., first 512 bytes)
- [ ] do() checks for io.EOF on empty body (empty error response is valid)
- [ ] do() wraps errors with operation context ("get item: status 500: server error")
- [ ] do() decodes JSON into the destination type
- [ ] do() returns ErrNotFound as-is (not wrapped)
- [ ] do() wraps StatusError and other errors with fmt.Errorf("%w", err)
- [ ] Client respects caller's context (timeout, cancellation)
- [ ] Client reuses connections (drains allow http.Client to reuse)
- [ ] Tests use httptest.NewServer (not a real network port)
- [ ] Tests verify GetItem with valid ID, 404 not found, server error
- [ ] Tests verify CreateItem with valid item, validation error, conflict
- [ ] Tests verify context cancellation (handler returns cancelled context)
- [ ] Tests verify context deadline (handler sleeps past deadline)
- [ ] Tests verify timeout on http.Client (slow server)
- [ ] Tests verify connection reuse (concurrent requests use fewer connections)
- [ ] Tests verify error wrapping (errors.Is works for ErrNotFound)

## Project: URL Shortener

### Readable Solution
The project implements a complete URL-shortener HTTP API with three main components:

1. **Server struct and NewServer**: Creates a Server with defaults.
   - If Store is nil, creates MemoryStore
   - If Logger is nil, creates slog.New(slog.NewDiscardHandler())
   - Trims trailing "/" from BaseURL
   - Initializes code generator (random string generator with collision retry)

2. **handleCreate** (POST /links): Validates and creates a shortened link.
   - Reads and validates Content-Type (application/json)
   - Caps request body at 4 KiB
   - Decodes request: {url, code (optional)}
   - Validates URL: must be http/https, non-empty, ≤2048 chars, has a host
   - If code provided: validates it (4-32 chars of [A-Za-z0-9_-])
   - If code omitted: generates random code, retries up to 5 times on collision
   - Creates Link struct with creation time (time.Now().UTC())
   - Calls Store.Create; returns 409 on ErrExists (code taken)
   - Returns 201 with Location header (e.g., "http://localhost:8080/abc123") and JSON body

3. **handleRedirect** (GET /{code}): Redirects to the original URL.
   - Extracts code from r.PathValue("code")
   - Calls Store.Visit (increments visits, returns updated Link)
   - Returns 302 Found with Location header pointing to original URL
   - Returns 404 if code not found

4. **handleStats** (GET /links/{code}/stats): Returns link statistics.
   - Extracts code from r.PathValue("code")
   - Calls Store.Get (does NOT increment visits)
   - Returns 200 with full Link JSON (code, url, short_url, visits, created_at)
   - Returns 404 if code not found

5. **validateURL**: Validates URLs.
   - Rejects empty URL
   - Rejects URL longer than MaxURLLen (2048)
   - Parses with url.Parse
   - Checks scheme is http or https
   - Checks host is non-empty (catches "/path", "http://")
   - Optional: reject localhost (for testing, you'd allow it)

6. **validCode**: Validates custom codes.
   - Checks length: 4-32 characters
   - Checks characters: [A-Za-z0-9_-] only
   - Case-sensitive

7. **Store interface and MemoryStore**:
   ```go
   import "gocourse/solutions/08-http/project/shortener"

   type Link = shortener.Link

   type Store interface {
   	Create(ctx context.Context, l Link) error // returns ErrExists if taken
   	Get(ctx context.Context, code string) (Link, error)
   	Visit(ctx context.Context, code string) (Link, error) // increments visits
   }

   func main() {
   	var s Store = shortener.NewMemoryStore()
   	ctx := context.Background()
   	s.Create(ctx, Link{Code: "go", URL: "https://go.dev"})
   	fmt.Println(s.Create(ctx, Link{Code: "go"})) // the code is taken: ErrExists
   	l, _ := s.Visit(ctx, "go")
   	fmt.Println(l.Visits) // 1
   }
   ```
   - MemoryStore uses sync.RWMutex to protect links map
   - Create: returns ErrExists if code already exists
   - Get: returns ErrNotFound if code doesn't exist
   - Visit: increments visits atomically, returns ErrNotFound if not found

8. **logRequests**: Middleware that logs every request.
   - Logs with method, path, status code, and duration
   - Uses logger.InfoContext to carry context attributes
   - Wraps handler with statusWriter to capture response status

### Optimized Solution
Could optimize by:
- Using sync.Map instead of RWMutex for MemoryStore (overkill for this scale)
- Using atomic.Int64 for visit counter (unnecessary; RWMutex is simple and clear)
- Pre-allocating links map with capacity hint (minor; Go's map growth is efficient)
- Caching validation regex if doing regex-based validation (current approach using url.Parse is simpler)

The current solution is already efficient. MemoryStore will handle thousands of links without bottleneck.

### Trade-off
- **Readable and optimized are the same** for this project.
- **Validation functions are explicit** (validateURL, validCode) for clarity and testability
- **Inlining would reduce readability** without meaningful performance gain
- **Use MemoryStore for development**, but the Store interface allows swapping to PostgreSQL/SQLite without changing handlers

### Review Checklist
- [ ] NewServer creates and returns *Server
- [ ] NewServer fills defaults: nil Store → new MemoryStore, nil Logger → discard handler
- [ ] NewServer trims trailing "/" from BaseURL
- [ ] handleCreate handles POST /links
- [ ] handleCreate validates Content-Type is application/json (415 on mismatch)
- [ ] handleCreate caps request body at 4 KiB with http.MaxBytesReader (413 on overflow)
- [ ] handleCreate decodes JSON with DisallowUnknownFields (400 on unknown field)
- [ ] handleCreate validates request body is not empty (400 on missing URL)
- [ ] handleCreate validates URL is http or https (400 on wrong scheme)
- [ ] handleCreate validates URL has a host (400 on missing host)
- [ ] handleCreate validates URL length ≤ 2048 (400 on too long)
- [ ] handleCreate rejects custom code if invalid (calls validCode; 400 on fail)
- [ ] handleCreate generates random code if omitted
- [ ] handleCreate retries generating codes on collision (up to 5 times; 500 if exhausted)
- [ ] handleCreate returns 201 Created (not 200)
- [ ] handleCreate sets Location header to short_url (e.g., "http://localhost/abc")
- [ ] handleCreate returns 409 Conflict if code already exists (Store.Create returns ErrExists)
- [ ] handleCreate encodes response as JSON {code, url, short_url, visits, created_at}
- [ ] handleRedirect handles GET /{code}
- [ ] handleRedirect extracts code from r.PathValue("code")
- [ ] handleRedirect calls Store.Visit (not Get; increments visits)
- [ ] handleRedirect returns 302 Found with Location header (permanent would be 301)
- [ ] handleRedirect returns 404 if code not found
- [ ] handleStats handles GET /links/{code}/stats
- [ ] handleStats extracts code from r.PathValue("code")
- [ ] handleStats calls Store.Get (not Visit; does not increment)
- [ ] handleStats returns 200 with full Link JSON
- [ ] handleStats returns 404 if code not found
- [ ] validateURL rejects empty URL (400)
- [ ] validateURL rejects URL > 2048 chars (400)
- [ ] validateURL parses URL and checks scheme (http/https only)
- [ ] validateURL checks host is non-empty (rejects "/path", "http://")
- [ ] validCode rejects code < 4 or > 32 chars (400)
- [ ] validCode rejects invalid characters (only [A-Za-z0-9_-])
- [ ] validCode is case-sensitive
- [ ] MemoryStore.Create stores Link and returns ErrExists if taken
- [ ] MemoryStore.Get returns Link and ErrNotFound if missing
- [ ] MemoryStore.Visit increments visits and returns updated Link
- [ ] MemoryStore uses sync.RWMutex to protect links map (concurrent safe)
- [ ] Visit counter increments only on redirect (GET /{code}), not on stats
- [ ] Generated codes are random and unpredictable (not sequential)
- [ ] Time fields use time.Now().UTC() (consistent timezone)
- [ ] JSON responses include created_at in RFC3339 format (time.Time marshals correctly)
- [ ] All errors are JSON {error: "message"}
- [ ] Internal errors (code generation, store failures) log details, send generic message
- [ ] All requests are logged with method, path, status, duration
- [ ] Tests cover: create with/without code, validation errors, collision, redirect, stats
- [ ] Tests cover: malformed JSON, unknown fields, body too large, wrong Content-Type
- [ ] Tests cover: concurrent creates (race detector passes)
- [ ] Tests use httptest.NewServer (no real network ports)
- [ ] All tests pass with -race flag

## Common Issues to Watch For

### Context Misuse
- **Using context.Background() in handlers** instead of r.Context()
  - Impact: Client cancellations are ignored; unnecessary work continues
  - Fix: Always use r.Context() for database queries, API calls, long operations
  
- **Not passing context to downstream calls**
  - Impact: Caller's timeout/cancellation doesn't propagate
  - Fix: Pass r.Context() to db.Query, client.GetItem, etc.
  
- **Sharing context across requests**
  - Impact: Values from one request leak into another; racy behavior
  - Fix: Derive child contexts in each request, don't reuse the parent

### Panic Handling
- **Panicking in handlers without recovery middleware**
  - Impact: Goroutine dies; client gets no response; connection is lost
  - Fix: Add Recover middleware, or validate input before operations that might panic
  
- **Panicking in spawned goroutines**
  - Impact: Panic in goroutine doesn't reach handler's recover()
  - Fix: Use error channels or WaitGroup; never assume panic is caught across goroutine boundaries
  
- **Forgetting to re-panic http.ErrAbortHandler**
  - Impact: Framework's abort mechanism is suppressed; connection management breaks
  - Fix: Defer recovery, and if recover() returns http.ErrAbortHandler, re-panic it

### JSON Encoding/Decoding
- **Writing status without buffering the response first**
  - Impact: If encoding fails partway, client sees partial body with wrong status
  - Fix: Encode to bytes.Buffer first, then write status and complete body atomically
  
- **Forgetting to validate Content-Type**
  - Impact: Client sends binary data claiming it's JSON; Decode silently fails or misinterprets
  - Fix: Use mime.ParseMediaType(r.Header.Get("Content-Type")) and check mediaType == "application/json"
  
- **Not capping request body size**
  - Impact: Client uploads 100 MB "JSON" and your process runs out of memory
  - Fix: Use http.MaxBytesReader(w, r.Body, maxSize) before Decode
  
- **Using Decode without checking for trailing values**
  - Impact: Client sends two JSON documents; only the first is decoded, second is silently ignored
  - Fix: After Decode, attempt to read one more token; if successful, it's an error
  
- **Not calling DisallowUnknownFields()**
  - Impact: Client typos in field names (user_ids vs user_id) are silently ignored
  - Fix: Call dec.DisallowUnknownFields() before Decode
  
- **Returning errors from json.Decoder without context**
  - Impact: Client sees opaque "unexpected character" errors
  - Fix: Wrap with context: fmt.Errorf("decode request: %w", err)

### Middleware
- **Applying middleware in the wrong order**
  - Impact: Auth runs before logging; unauthorized requests aren't logged
  - Fix: Chain(h, Logging, Auth) applies Logging outermost (runs first on request)
  
- **Forgetting to call next.ServeHTTP**
  - Impact: Handler never runs; request hangs or times out
  - Fix: Every middleware must call next.ServeHTTP(w, r) (or return early on error)
  
- **Sharing mutable state across requests without synchronization**
  - Impact: Race conditions in concurrent requests
  - Fix: Use request context to carry request-scoped data; use sync.Mutex for shared state
  
- **Not implementing Unwrap() on wrapper types**
  - Impact: Code that tries to access underlying writer (via http.ResponseController) fails
  - Fix: Wrapper types should implement Unwrap() http.ResponseWriter
  
- **Logging sensitive data (passwords, API keys)**
  - Impact: Logs leak credentials; compliance violations
  - Fix: Filter context values before logging; sanitize request/response bodies

### HTTP Client
- **Using context.Background() instead of passing caller's context**
  - Impact: Client request ignores caller's timeout/cancellation
  - Fix: Use http.NewRequestWithContext(ctx, ...) where ctx is the caller's context
  
- **Not draining response bodies**
  - Impact: TCP connections aren't returned to pool; socket leak; resource exhaustion
  - Fix: Always defer io.Copy(io.Discard, resp.Body); resp.Body.Close()
  
- **Not setting a timeout on http.Client**
  - Impact: Goroutine hangs forever if server never responds
  - Fix: client := &http.Client{Timeout: 10*time.Second}
  
- **Comparing status code with == instead of checking range**
  - Impact: Checking resp.StatusCode == 200 fails on valid 201/204/etc responses
  - Fix: Check StatusCode explicitly for the expected value; wrap non-matching codes as errors
  
- **Ignoring write errors when draining**
  - Impact: Silent failures in cleanup
  - Fix: Acceptable to ignore only when draining (client is gone); always check other errors
  
- **Not reading response body after error**
  - Impact: Error details lost; harder to debug
  - Fix: Read (truncated) body on non-2xx status codes for error message

### HTTP Handlers & Routing
- **Writing status without setting Content-Type**
  - Impact: Browser treats response as HTML or binary; client can't parse response
  - Fix: Set Content-Type before WriteHeader: w.Header().Set("Content-Type", "application/json")
  
- **Calling WriteHeader twice**
  - Impact: Second call is silently ignored; unexpected status or headers
  - Fix: Write status once; use helpers to ensure atomic write
  
- **Assuming path parameters are non-empty**
  - Impact: Panics or crashes on edge cases (empty string, negative numbers)
  - Fix: Validate extracted parameters before using them
  
- **Not using /{$} for exact root path**
  - Impact: GET / matches "", which is a catch-all for unregistered paths
  - Fix: Register GET /{$} for exact match; mux tries more specific patterns first
  
- **Forgetting method in pattern**
  - Impact: Pattern matches all methods (GET, POST, DELETE, etc.)
  - Fix: Always include method: "GET /items", "POST /items", "DELETE /items/{id}"
  
- **Not checking specific error types from database/upstream**
  - Impact: 500 responses for client errors (validation, not found)
  - Fix: Switch on error types; return 400 for validation, 404 for not found

### Race Conditions & Concurrency
- **Sharing maps or slices across requests without sync.Mutex**
  - Impact: Concurrent requests corrupt data; go test -race detects it
  - Fix: Protect shared state with sync.RWMutex or use channels
  
- **Recording request-scoped data in global variables**
  - Impact: User A's data appears in User B's response
  - Fix: Use context.WithValue to carry request data; never use global state
  
- **Goroutine spawned in handler doesn't respect cancellation**
  - Impact: Goroutine continues running after client disconnects or timeout fires
  - Fix: Pass r.Context() to goroutine; check ctx.Done() or use sync.WaitGroup with timeout
  
- **Calling cancel() multiple times**
  - Impact: Panic on second cancel
  - Fix: Defer cancel() once; never call it multiple times in one scope

### Testing
- **Using real network ports in tests (listening on :8080)**
  - Impact: Tests interfere with each other; port conflicts; flaky on busy systems
  - Fix: Use httptest.NewServer (dynamic port) or httptest.NewRecorder (no network)
  
- **Not using httptest utilities**
  - Impact: Manual socket setup; tests are slow and fragile
  - Fix: httptest.NewServer for integration; httptest.NewRecorder for unit tests
  
- **Not testing error cases (validation, 404, 500)**
  - Impact: Hidden bugs in error paths; poor test coverage
  - Fix: Create subtests for each error case; verify status code and error message
  
- **Not running tests with -race**
  - Impact: Race conditions hide until production
  - Fix: Run go test -race ./... in CI; catch data races early
  
- **Assuming specific error messages from upstream**
  - Impact: Tests break if upstream error message changes
  - Fix: Check error type (errors.Is) or status code; don't assert on message text
