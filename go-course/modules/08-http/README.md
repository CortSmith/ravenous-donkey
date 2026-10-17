# Module 08: Context, HTTP Servers & Clients

## Goals

By the end of this module, you will be able to:

- Write net/http handlers using Go 1.22 ServeMux patterns with method routing and wildcard path parameters
- Use context for cancellation, deadlines, and request-scoped values
- Implement middleware chains and composable request processing with request IDs and logging
- Encode and decode JSON safely in HTTP handlers with validation
- Write HTTP clients that respect timeouts, drain response bodies, and handle errors correctly
- Gracefully handle panics in HTTP handlers and log them
- Write tests for HTTP handlers and clients using httptest without any real network ports

## Concepts

### Context in HTTP handlers

The `*http.Request` carries a context obtained with `r.Context()`. This context:
- Is canceled when the client disconnects
- Carries request-scoped values
- Can have timeouts and deadlines applied by the framework

You should always use the request context when making calls that might block (database queries, API calls, etc.). That way, if the client times out or cancels their request, your handler can give up gracefully.

```go
// A handler that respects the request context
func (h *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
    // The context is already set up with timeouts by the HTTP server
    user, err := h.db.GetUser(r.Context(), userID)
    if err != nil {
        // If the client cancelled, ctx.Err() is context.Canceled
        // If a timeout fired, ctx.Err() is context.DeadlineExceeded
        return fmt.Errorf("get user: %w", err)
    }
    // ...
}
```

You can also add your own deadlines or derive child contexts:

```go
// Add a 1-second timeout to a health check
ctx, cancel := context.WithTimeout(r.Context(), time.Second)
defer cancel()
if err := checkDatabase(ctx); err != nil {
    return fmt.Errorf("database is unhealthy: %w", err)
}
```

Context values are thread-safe and should be used for request-scoped data (trace IDs, user IDs, etc.), not shared state.

### net/http ServeMux (Go 1.22+)

Go 1.22 brought method-aware patterns and wildcard path parameters to the standard `http.ServeMux`. Gone are the days of manually checking `r.Method` or parsing path segments:

```go
mux := http.NewServeMux()

// Method-specific patterns
mux.HandleFunc("GET /items", h.listItems)      // only GET requests
mux.HandleFunc("POST /items", h.createItem)    // only POST requests
mux.HandleFunc("GET /items/{id}", h.getItem)   // matches "GET /items/123"
mux.HandleFunc("DELETE /items/{id}", h.deleteItem)

// Wildcard pattern (matches rest of path)
mux.HandleFunc("GET /files/{path...}", h.serveFile) // "GET /files/a/b/c.txt" -> r.PathValue("path") == "a/b/c.txt"

// Exact root path only. A plain "/" pattern matches EVERY path (it is a
// prefix pattern); "{$}" anchors it so only "/" itself matches.
mux.HandleFunc("GET /{$}", h.serveRoot)
```

Key rules:
- Patterns are case-sensitive
- `{id}` captures a single path segment (everything up to the next `/`)
- `{path...}` captures the rest of the path (may contain `/`)
- More specific patterns win regardless of registration order (e.g., `/items/latest` beats `/items/{id}`); registering two patterns that conflict with neither more specific panics at registration
- A `GET` pattern also matches `HEAD` requests
- POST to a GET-only route gets an automatic 405 with an `Allow` header
- `r.PathValue("id")` retrieves a captured segment

### JSON encoding/decoding in HTTP handlers

When writing JSON responses, encode into a buffer first. That way, if encoding fails, the client gets a 500 *before* any response body is sent. If you write the status and body piecemeal, a write error leaves the client with a partial response.

```go
// GOOD: buffer first, write atomically
var buf bytes.Buffer
if err := json.NewEncoder(&buf).Encode(resp); err != nil {
    http.Error(w, "internal server error", http.StatusInternalServerError)
    return err
}
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusCreated)
w.Write(buf.Bytes())

// BAD: write header and body separately; if Encode fails midway, client sees partial data
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusCreated)
if err := json.NewEncoder(w).Encode(resp); err != nil { ... } // too late!
```

When reading JSON request bodies:
- Always set `Content-Type` requirements and validate with `mime.ParseMediaType`
- Use `http.MaxBytesReader` to cap the body size before decoding
- Call `DisallowUnknownFields()` on the decoder to reject unrecognized fields
- Check for a second JSON value (multiple documents)
- Distinguish client errors (400) from server errors (500)

```go
// Typical handler receiving JSON
func handleCreateItem(w http.ResponseWriter, r *http.Request) {
    // Check Content-Type
    mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
    if mediaType != "application/json" {
        http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
        return
    }

    // Cap body size
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()

    var item Item
    if err := dec.Decode(&item); err != nil {
        // Distinguish client errors
        if errors.Is(err, io.EOF) {
            http.Error(w, "body is empty", http.StatusBadRequest)
        } else {
            // inspect err to craft a helpful message
            http.Error(w, "malformed JSON", http.StatusBadRequest)
        }
        return
    }
    // ...
}
```

### Middleware chaining

Middleware is a function that wraps a handler. By chaining middleware, you can add cross-cutting concerns (logging, auth, panics recovery) without modifying handler code.

```go
// Middleware type: takes a handler, returns a handler
type Middleware func(http.Handler) http.Handler

// Chain applies middleware from outside to inside
// If mws = [a, b, c], the order is a -> b -> c -> handler
func Chain(h http.Handler, mws ...Middleware) http.Handler {
    for i := len(mws) - 1; i >= 0; i-- {
        h = mws[i](h)
    }
    return h
}

// Example: request ID middleware
func RequestID(gen func() string) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id := r.Header.Get("X-Request-ID")
            if id == "" {
                id = gen()
            }
            w.Header().Set("X-Request-ID", id)
            ctx := context.WithValue(r.Context(), ctxKeyID, id)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// statusWriter remembers the status code so middleware can log it and
// knows whether the response has started.
type statusWriter struct {
    http.ResponseWriter
    status      int
    wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
    if !w.wroteHeader {
        w.status, w.wroteHeader = code, true
    }
    w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
    if !w.wroteHeader {
        w.status, w.wroteHeader = http.StatusOK, true // implicit 200
    }
    return w.ResponseWriter.Write(p)
}

// Example: logging middleware
func Logging(logger *slog.Logger) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            sw := &statusWriter{ResponseWriter: w}
            next.ServeHTTP(sw, r)
            logger.InfoContext(r.Context(), "request",
                "method", r.Method,
                "path", r.URL.Path,
                "status", sw.status,
                "duration", time.Since(start),
            )
        })
    }
}

// Use them
mux := http.NewServeMux()
// ... register handlers ...
handler := Chain(mux,
    Logging(logger),
    RequestID(rand.Text), // crypto/rand.Text (Go 1.24): a random 26-character string
)
http.ListenAndServe(":8080", handler)
```

The key insight: middleware runs in nested order, so the outermost middleware sees the entire request/response lifecycle.

### HTTP client best practices

A well-behaved HTTP client:
1. Uses a timeout on the http.Client
2. Passes context on every request
3. Checks the status code explicitly
4. Drains the response body even on error (so connections are reused)
5. Wraps errors with context

```go
type Client struct {
    baseURL string
    http    *http.Client
}

// Timeout applies to the entire request+response, not just the connection
func New(baseURL string, timeout time.Duration) *Client {
    return &Client{
        baseURL: baseURL,
        http: &http.Client{Timeout: timeout},
    }
}

func (c *Client) GetItem(ctx context.Context, id string) (Item, error) {
    // PathEscape: an id like "a/b" or "x?y" must not change the URL's shape.
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/items/"+url.PathEscape(id), nil)
    if err != nil {
        return Item{}, fmt.Errorf("get item: %w", err)
    }

    resp, err := c.http.Do(req)
    if err != nil {
        return Item{}, fmt.Errorf("get item: %w", err)
    }
    defer resp.Body.Close()

    // Always drain to allow connection reuse
    defer io.Copy(io.Discard, resp.Body)

    // Check status explicitly
    if resp.StatusCode != http.StatusOK {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 512)) // don't trust the size of an error body
        return Item{}, fmt.Errorf("get item: status %d: %s", resp.StatusCode, string(b))
    }

    var item Item
    if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
        return Item{}, fmt.Errorf("get item: decode: %w", err)
    }
    return item, nil
}
```

### Panic recovery in HTTP

If a handler panics, `net/http` recovers the panic itself, logs a stack trace, and closes the connection, so the server survives but the client gets no response. To log it your way and send a 500 instead:

```go
func Recover(logger *slog.Logger) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            sw := &statusWriter{ResponseWriter: w}
            defer func() {
                v := recover()
                if v == nil {
                    return
                }
                // http.ErrAbortHandler is the signal to abort; re-panic it
                if v == http.ErrAbortHandler {
                    panic(v)
                }
                // Log and send 500
                logger.ErrorContext(r.Context(), "panic recovered",
                    "panic", fmt.Sprint(v),
                    "path", r.URL.Path,
                )
                if !sw.wroteHeader {
                    http.Error(w, "internal server error", http.StatusInternalServerError)
                }
            }()
            next.ServeHTTP(sw, r)
        })
    }
}
```

If the handler already started writing, the status line has been sent and it
is too late for a 500. All you can do is log.

## Idioms & Best Practices

- **Always pass context to backends**: Use `r.Context()` for database queries, external APIs, etc. The context carries client-cancellation and deadline information.
  ```go
  // GOOD
  user, err := db.GetUser(r.Context(), id)
  
  // BAD
  user, err := db.GetUser(context.Background(), id)
  ```

- **Don't panic in handlers; recover and log**: Panics kill the goroutine but not the server. If you must panic, make it part of your recovery tests.
  ```go
  // GOOD: validate and return an error
  if id < 0 {
      http.Error(w, "id must be positive", http.StatusBadRequest)
      return
  }
  
  // BAD
  if id < 0 {
      panic("invalid id") // client gets no response; hard to debug
  }
  ```

- **Drain response bodies even on error**: Without draining, the HTTP connection can't be reused.
  ```go
  defer func() {
      io.Copy(io.Discard, resp.Body)
      resp.Body.Close()
  }()
  // Now safe to check status and decode
  ```

- **Encode response bodies into a buffer first**: If encoding fails partway through, you can't send a 500 (headers are already sent).
  ```go
  // GOOD
  var buf bytes.Buffer
  if err := json.NewEncoder(&buf).Encode(resp); err != nil { ... }
  w.WriteHeader(http.StatusCreated)
  w.Write(buf.Bytes())
  
  // BAD
  w.WriteHeader(http.StatusCreated)
  json.NewEncoder(w).Encode(resp) // if Encode fails, client sees 201 and partial body
  ```

- **Validate Content-Type and body size**: Use `mime.ParseMediaType` and `http.MaxBytesReader`.
  ```go
  mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
  if mediaType != "application/json" {
      return fmt.Errorf("want application/json, got %s", mediaType)
  }
  r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
  ```

- **Use `DisallowUnknownFields()` for API inputs**: This catches typos and API versioning issues.
  ```go
  dec := json.NewDecoder(r.Body)
  dec.DisallowUnknownFields()
  if err := dec.Decode(&req); err != nil { ... }
  ```

- **Use `time.Duration` and `time.Time` carefully**: Duration is in nanoseconds; time.Time has a monotonic component that's excluded from JSON. For comparisons, use `t1.Equal(t2)`, not `t1 == t2`.

- **Log with context, not globally**: Use `logger.InfoContext(r.Context(), ...)` so that request-scoped values (trace ID) are included automatically.
  ```go
  logger.InfoContext(r.Context(), "request processed",
      "user_id", userID,
      "status", statusCode,
  )
  ```

- **Chain middleware from outermost to innermost**: The order matters. Logging should wrap authentication so you log all requests, not just successful ones.
  ```go
  // Request → Logging → Auth → Handler
  handler := Chain(mux, Logging(logger), Auth(authFn))
  ```

## Common Pitfalls

- **Forgetting to use the request context**: If you use `context.Background()` in a handler, client cancellations are ignored, and you might do unnecessary work.

- **Sending a response header, then failing to encode**: Once `WriteHeader` or the first `Write` is called, you can't change the status. If encoding fails after that, you're stuck.

- **Not draining response bodies**: Without draining, connections aren't reused, and you leak resources. Even if you only need the status code, drain the body.

- **Panicking in a goroutine spawned by a handler**: If a goroutine panics, the panic doesn't reach the handler's defer recover(). Use error channels instead.
  ```go
  // BAD: panic in goroutine won't be caught
  go func() { panic("oops") }()
  
  // GOOD: use a channel
  errCh := make(chan error, 1)
  go func() { errCh <- doSomething() }()
  if err := <-errCh; err != nil { ... }
  ```

- **Sharing request-scoped data via global variables**: Use `context.WithValue` to carry request data.
  ```go
  // BAD
  var userID int // global, racy
  
  // GOOD
  ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
  ```

- **Not checking errors from `Close()` or `Flush()` on writers**: for a buffered or file writer, that error is often the *only* report that data never reached its destination.
  ```go
  // BAD: the error that matters is thrown away
  defer bw.Flush()

  // GOOD
  if err := bw.Flush(); err != nil {
      return fmt.Errorf("flush: %w", err)
  }
  ```

- **Encoding nil slices and maps as JSON**: a nil slice encodes as `null`, not `[]`, and a nil map as `null`, not `{}`. Clients written in other languages often choke on `null` where they expect a list.
  ```go
  // BAD: {"items":null} when there are no items
  resp := ListResponse{Items: items} // items may be nil

  // GOOD: {"items":[]}
  if items == nil {
      items = []Item{}
  }
  resp := ListResponse{Items: items}
  ```

## Exercises

### Exercise 1: Health Handler
**Package**: `health`

Implement a health-check endpoint that runs a set of named checks, each under its own timeout derived from the request context. Stretch: make the endpoint's latency that of the slowest check instead of the sum of all checks.

Implement:
- `Handler(timeout, checks)` → `http.Handler` that runs every check under its own deadline and returns 200 if all pass, 503 otherwise

**Test command:**
```sh
go test ./modules/08-http/exercises/health/
```

### Exercise 2: JSON I/O Helpers
**Package**: `jsonio`

Write strict JSON encoding/decoding helpers for HTTP handlers that validate Content-Type, cap request bodies, disallow unknown fields, and encode responses into a buffer first.

Implement:
- `WriteJSON(w, status, v)` → encodes and writes JSON with the given status
- `WriteError(w, status, msg)` → writes `{"error": msg}` as JSON
- `DecodeJSON(w, r, dst)` → decodes the request body as JSON, returning a `*RequestError` on validation failures

**Test command:**
```sh
go test ./modules/08-http/exercises/jsonio/
```

### Exercise 3: Middleware Chain
**Package**: `middleware`

Implement middleware for request IDs (carried in context), structured logging with slog, and panic recovery.

Implement:
- `Chain(h, mws...)` → wraps h with middleware, applying them from outside to inside
- `RequestID(newID)` → middleware that assigns each request a unique ID from the X-Request-ID header or a generated one
- `Logging(logger)` → middleware that logs every request with method, path, status, bytes written, and duration
- `Recover(logger)` → middleware that catches panics, logs them, and sends 500 (unless the handler already started the response)

**Test command:**
```sh
go test ./modules/08-http/exercises/middleware/
```

### Exercise 4: Path Parameter Routing
**Package**: `routes`

Use Go 1.22 ServeMux patterns to expose a notes store over HTTP with method and wildcard routing.

Implement:
- `Routes(s *Store)` → returns an `http.Handler` with routes for listing, creating, getting, and deleting notes

The store is provided complete; the exercise is the HTTP layer.

**Test command:**
```sh
go test ./modules/08-http/exercises/routes/
```

### Exercise 5: HTTP Client
**Package**: `client`

Write a well-behaved HTTP client that respects timeouts, drains response bodies, checks status codes, and wraps errors.

Implement:
- `New(baseURL, timeout)` → returns a Client with the given base URL and timeout
- `GetItem(ctx, id)` → fetches GET {base}/items/{id}, returning a 404 as `ErrNotFound`
- `CreateItem(ctx, it)` → sends POST {base}/items with a JSON body, expecting 201

**Test command:**
```sh
go test ./modules/08-http/exercises/client/
```

## Project: URL Shortener API

A complete URL-shortener HTTP API with routes to create links, redirect, and fetch statistics, backed by an in-memory store.

**Routes:**
- `POST /links` — create a short link; expects `{"url": "https://..."}`, optionally `"code"` for custom short codes
- `GET /{code}` — redirect to the original URL
- `GET /links/{code}/stats` — fetch statistics (visits, creation time)

**Requirements:**
- Use ServeMux patterns for routing
- Validate URLs and custom codes
- Implement a Store interface so the HTTP layer doesn't depend on the storage implementation
- Use httptest for tests; no real ports
- Gracefully log and handle errors

**Spec:** See `project/README.md`.

## Submitting for Review

When all exercise tests pass and the project is complete, ask your instructor (Claude) to review `modules/08-http/`. The review will cover:
- Correct use of ServeMux routing and path parameters
- Proper context propagation and cancellation
- Safe JSON encoding/decoding in handlers
- Effective middleware composition
- Connection reuse in the HTTP client
- Error handling and logging
- Graceful error recovery and panic handling

## Further Reading

- **Go 1.22 net/http enhancements:** https://go.dev/blog/routing-enhancements
- **Context package:** https://pkg.go.dev/context
- **ServeMux:** https://pkg.go.dev/net/http#ServeMux
- **Effective Go, "A web server":** https://go.dev/doc/effective_go#web_server
- **json.Decoder:** https://pkg.go.dev/encoding/json#Decoder
- **http.Client:** https://pkg.go.dev/net/http#Client
- **httptest:** https://pkg.go.dev/net/http/httptest
- **slog structured logging:** https://pkg.go.dev/log/slog
- **Go Code Review Comments:** https://go.dev/wiki/CodeReviewComments
