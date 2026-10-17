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
type User struct{ ID, Name string }

type DB struct{}

func (DB) GetUser(ctx context.Context, id string) (User, error) {
	select {
	case <-time.After(50 * time.Millisecond): // a slow query
		return User{ID: id, Name: "Ada"}, nil
	case <-ctx.Done(): // ...that stops when the request does
		return User{}, ctx.Err()
	}
}

type Server struct{ db DB }

// A handler that respects the request context
func (h *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	// The server cancels r.Context() when the client goes away
	user, err := h.db.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		// If the client cancelled, err is context.Canceled
		// If a timeout fired, err is context.DeadlineExceeded
		http.Error(w, "get user: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, user.Name)
}

func main() {
	h := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", h.handleGetUser)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/users/1", nil))
	fmt.Print(rec.Code, " ", rec.Body.String()) // 200 Ada

	// A client that gives up after 10ms:
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, "GET", "/users/1", nil))
	fmt.Print(rec.Code, " ", rec.Body.String()) // 503 get user: context deadline exceeded
}
```

You can also add your own deadlines or derive child contexts:

```go
func checkDatabase(ctx context.Context) error {
	select {
	case <-time.After(2 * time.Second): // a database that hangs
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

health := func(w http.ResponseWriter, r *http.Request) {
	// Add a 1-second timeout to a health check
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := checkDatabase(ctx); err != nil {
		http.Error(w, fmt.Sprintf("database is unhealthy: %v", err), http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "ok")
}

rec := httptest.NewRecorder()
health(rec, httptest.NewRequest("GET", "/healthz", nil)) // takes 1s, not 2s
fmt.Print(rec.Code, " ", rec.Body.String())              // 503 database is unhealthy: context deadline exceeded
```

Context values are thread-safe and should be used for request-scoped data (trace IDs, user IDs, etc.), not shared state.

### net/http ServeMux (Go 1.22+)

Go 1.22 brought method-aware patterns and wildcard path parameters to the standard `http.ServeMux`. Gone are the days of manually checking `r.Method` or parsing path segments:

```go
reply := func(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s id=%q path=%q", name, r.PathValue("id"), r.PathValue("path"))
	}
}

mux := http.NewServeMux()

// Method-specific patterns
mux.HandleFunc("GET /items", reply("listItems"))        // only GET requests
mux.HandleFunc("POST /items", reply("createItem"))      // only POST requests
mux.HandleFunc("GET /items/{id}", reply("getItem"))     // matches "GET /items/123"
mux.HandleFunc("DELETE /items/{id}", reply("deleteItem"))

// Wildcard pattern (matches rest of path)
mux.HandleFunc("GET /files/{path...}", reply("serveFile")) // "GET /files/a/b/c.txt" -> r.PathValue("path") == "a/b/c.txt"

// Exact root path only. A plain "/" pattern matches EVERY path (it is a
// prefix pattern); "{$}" anchors it so only "/" itself matches.
mux.HandleFunc("GET /{$}", reply("serveRoot"))

for _, req := range []string{"GET /items/123", "DELETE /items/9", "GET /files/a/b/c.txt", "GET /", "PUT /items", "GET /nope"} {
	method, path, _ := strings.Cut(req, " ")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	fmt.Printf("%-22s -> %d %s\n", req, rec.Code, strings.TrimSpace(rec.Body.String()))
}
// PUT /items gets 405 Method Not Allowed; GET /nope gets 404
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
type response struct {
	ID    int `json:"id"`
	Price any `json:"price"`
}

// GOOD: buffer first, write atomically
good := func(w http.ResponseWriter, resp response) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(buf.Bytes())
}

// BAD: write header and body separately; if Encode fails midway, client sees partial data
bad := func(w http.ResponseWriter, resp response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError) // too late!
	}
}

broken := response{ID: 1, Price: math.Inf(1)} // JSON can't encode +Inf
rec := httptest.NewRecorder()
good(rec, broken)
fmt.Println("good:", rec.Code) // good: 500
rec = httptest.NewRecorder()
bad(rec, broken)
fmt.Println("bad:", rec.Code) // bad: 201, for a request that failed
```

When reading JSON request bodies:
- Always set `Content-Type` requirements and validate with `mime.ParseMediaType`
- Use `http.MaxBytesReader` to cap the body size before decoding
- Call `DisallowUnknownFields()` on the decoder to reject unrecognized fields
- Check for a second JSON value (multiple documents)
- Distinguish client errors (400) from server errors (500)

```go
type Item struct {
	Name  string `json:"name"`
	Price int    `json:"price"`
}

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
			http.Error(w, "malformed JSON: "+err.Error(), http.StatusBadRequest)
		}
		return
	}
	fmt.Fprintf(w, "created %+v\n", item)
}

func main() {
	for _, body := range []string{`{"name": "pen", "price": 3}`, ``, `{"name": "pen", "colour": "red"}`} {
		req := httptest.NewRequest("POST", "/items", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		rec := httptest.NewRecorder()
		handleCreateItem(rec, req)
		fmt.Print(rec.Code, " ", rec.Body.String())
	}
}
```

### Middleware chaining

Middleware is a function that wraps a handler. By chaining middleware, you can add cross-cutting concerns (logging, auth, panics recovery) without modifying handler code.

```go
import "crypto/rand"

// ctxKey is unexported, so no other package can collide with these keys.
type ctxKey int

const ctxKeyID ctxKey = 0

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
func main() {
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    mux := http.NewServeMux()
    mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "hello, request %s\n", r.Context().Value(ctxKeyID))
    })
    handler := Chain(mux,
        Logging(logger),
        RequestID(rand.Text), // crypto/rand.Text (Go 1.24): a random 26-character string
    )
    // A real server: http.ListenAndServe(":8080", handler). Here, one request in memory:
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, httptest.NewRequest("GET", "/hello", nil))
    fmt.Print(rec.Code, " ", rec.Body.String())
}
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
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

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

func main() {
	// A fake API server to talk to
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items/42" {
			http.Error(w, "no such item", http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"id": "42", "name": "pen"}`)
	}))
	defer srv.Close()

	c := New(srv.URL, 2*time.Second)
	fmt.Println(c.GetItem(context.Background(), "42"))  // {42 pen} <nil>
	fmt.Println(c.GetItem(context.Background(), "a/b")) // get item: status 404: no such item
}
```

### Panic recovery in HTTP

If a handler panics, `net/http` recovers the panic itself, logs a stack trace, and closes the connection, so the server survives but the client gets no response. To log it your way and send a 500 instead:

```go
type Middleware func(http.Handler) http.Handler

// statusWriter, from the middleware example above
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
		w.status, w.wroteHeader = http.StatusOK, true
	}
	return w.ResponseWriter.Write(p)
}

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

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	boom := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]int
		m["x"]++ // a bug: nil map
	})
	rec := httptest.NewRecorder()
	Recover(logger)(boom).ServeHTTP(rec, httptest.NewRequest("GET", "/boom", nil))
	fmt.Print(rec.Code, " ", rec.Body.String()) // 500 internal server error
}
```

If the handler already started writing, the status line has been sent and it
is too late for a 500. All you can do is log.

## Idioms & Best Practices

- **Always pass context to backends**: Use `r.Context()` for database queries, external APIs, etc. The context carries client-cancellation and deadline information.
  ```go
  type DB struct{}

  func (DB) GetUser(ctx context.Context, id int) (string, error) {
  	select {
  	case <-time.After(100 * time.Millisecond):
  		return "Ada", nil
  	case <-ctx.Done():
  		return "", ctx.Err()
  	}
  }

  var db DB
  handler := func(w http.ResponseWriter, r *http.Request) {
  	id := 1
  	// GOOD: stops when the client goes away
  	user, err := db.GetUser(r.Context(), id)
  	// BAD: keeps working for a client that is gone
  	//   user, err := db.GetUser(context.Background(), id)
  	fmt.Println(user, err)
  }

  ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond) // an impatient client
  defer cancel()
  handler(httptest.NewRecorder(), httptest.NewRequestWithContext(ctx, "GET", "/", nil)) // context deadline exceeded
  ```

- **Don't panic in handlers; recover and log**: Panics kill the goroutine but not the server. If you must panic, make it part of your recovery tests.
  ```go
  handler := func(w http.ResponseWriter, r *http.Request) {
  	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
  	// GOOD: validate and return an error
  	if id < 0 {
  		http.Error(w, "id must be positive", http.StatusBadRequest)
  		return
  	}
  	// BAD would be panic("invalid id"): the client gets no useful response
  	fmt.Fprintln(w, "item", id)
  }

  rec := httptest.NewRecorder()
  handler(rec, httptest.NewRequest("GET", "/item?id=-3", nil))
  fmt.Print(rec.Code, " ", rec.Body.String()) // 400 id must be positive
  ```

- **Drain response bodies even on error**: Without draining, the HTTP connection can't be reused.
  ```go
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  	fmt.Fprint(w, `{"ok": true}`)
  }))
  defer srv.Close()

  resp, err := http.Get(srv.URL)
  if err != nil {
  	log.Fatal(err)
  }
  defer func() {
  	io.Copy(io.Discard, resp.Body) // drain, so the connection can be reused
  	resp.Body.Close()
  }()
  // Now safe to check status and decode
  var out struct{ OK bool }
  err = json.NewDecoder(resp.Body).Decode(&out)
  fmt.Println(resp.StatusCode, out.OK, err) // 200 true <nil>
  ```

- **Encode response bodies into a buffer first**: If encoding fails partway through, you can't send a 500 (headers are already sent).
  ```go
  resp := map[string]any{"total": math.NaN()} // JSON can't encode NaN

  // GOOD
  good := httptest.NewRecorder()
  var buf bytes.Buffer
  if err := json.NewEncoder(&buf).Encode(resp); err != nil {
  	http.Error(good, "internal error", http.StatusInternalServerError)
  } else {
  	good.WriteHeader(http.StatusCreated)
  	good.Write(buf.Bytes())
  }

  // BAD
  bad := httptest.NewRecorder()
  bad.WriteHeader(http.StatusCreated)
  json.NewEncoder(bad).Encode(resp) // if Encode fails, client sees 201 and partial body

  fmt.Println(good.Code, bad.Code) // 500 201
  ```

- **Validate Content-Type and body size**: Use `mime.ParseMediaType` and `http.MaxBytesReader`.
  ```go
  func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
  	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
  	if mediaType != "application/json" {
  		return fmt.Errorf("want application/json, got %s", mediaType)
  	}
  	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
  	return json.NewDecoder(r.Body).Decode(v)
  }

  func main() {
  	var v map[string]any
  	req := httptest.NewRequest("POST", "/", strings.NewReader("a=1"))
  	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
  	fmt.Println(decodeJSON(httptest.NewRecorder(), req, &v)) // want application/json, got application/x-www-form-urlencoded
  }
  ```

- **Use `DisallowUnknownFields()` for API inputs**: This catches typos and API versioning issues.
  ```go
  var req struct{ Name string }
  r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name": "pen", "nmae": "typo"}`))

  dec := json.NewDecoder(r.Body)
  dec.DisallowUnknownFields()
  if err := dec.Decode(&req); err != nil {
  	fmt.Println(err) // json: unknown field "nmae": a typo you'd otherwise never notice
  }
  ```

- **Use `time.Duration` and `time.Time` carefully**: Duration is in nanoseconds; time.Time has a monotonic component that's excluded from JSON. For comparisons, use `t1.Equal(t2)`, not `t1 == t2`.

- **Log with context, not globally**: Use `logger.InfoContext(r.Context(), ...)` so that request-scoped values (trace ID) are included automatically.
  ```go
  logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
  r := httptest.NewRequest("GET", "/items", nil)
  userID, statusCode := 42, http.StatusOK

  logger.InfoContext(r.Context(), "request processed",
  	"user_id", userID,
  	"status", statusCode,
  )
  ```

- **Chain middleware from outermost to innermost**: The order matters. Logging should wrap authentication so you log all requests, not just successful ones.
  ```go
  type Middleware func(http.Handler) http.Handler

  func Chain(h http.Handler, mws ...Middleware) http.Handler {
  	for i := len(mws) - 1; i >= 0; i-- {
  		h = mws[i](h)
  	}
  	return h
  }

  func Logging(next http.Handler) http.Handler {
  	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  		fmt.Println("logging:", r.URL.Path)
  		next.ServeHTTP(w, r)
  	})
  }

  func Auth(next http.Handler) http.Handler {
  	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  		if r.Header.Get("Authorization") == "" {
  			fmt.Println("auth: rejected")
  			http.Error(w, "unauthorized", http.StatusUnauthorized)
  			return
  		}
  		next.ServeHTTP(w, r)
  	})
  }

  func main() {
  	mux := http.NewServeMux()
  	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Println("handler") })

  	// Request → Logging → Auth → Handler
  	handler := Chain(mux, Logging, Auth)

  	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/secret", nil))
  	req := httptest.NewRequest("GET", "/secret", nil)
  	req.Header.Set("Authorization", "Bearer token")
  	handler.ServeHTTP(httptest.NewRecorder(), req)
  }
  ```

## Common Pitfalls

- **Forgetting to use the request context**: If you use `context.Background()` in a handler, client cancellations are ignored, and you might do unnecessary work.

- **Sending a response header, then failing to encode**: Once `WriteHeader` or the first `Write` is called, you can't change the status. If encoding fails after that, you're stuck.

- **Not draining response bodies**: Without draining, connections aren't reused, and you leak resources. Even if you only need the status code, drain the body.

- **Panicking in a goroutine spawned by a handler**: If a goroutine panics, the panic doesn't reach the handler's defer recover(). Use error channels instead.
  ```go
  doSomething := func() error { return errors.New("failed in the background") }

  // BAD: a panic in this goroutine is not caught by the server's (or your
  // Recover middleware's) recover: it crashes the whole process.
  //
  //	go func() { panic("oops") }()

  // GOOD: use a channel
  errCh := make(chan error, 1)
  go func() { errCh <- doSomething() }()
  if err := <-errCh; err != nil {
  	fmt.Println("handled:", err)
  }
  ```

- **Sharing request-scoped data via global variables**: Use `context.WithValue` to carry request data.
  ```go
  // BAD
  //
  //	var userID int // global, racy: every request overwrites it

  // GOOD: a request-scoped value travels in the request's context
  type ctxKey int

  const ctxKeyUserID ctxKey = 0

  r := httptest.NewRequest("GET", "/", nil)
  userID := 42
  ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
  r = r.WithContext(ctx)
  fmt.Println(r.Context().Value(ctxKeyUserID)) // 42
  ```

- **Not checking errors from `Close()` or `Flush()` on writers**: for a buffered or file writer, that error is often the *only* report that data never reached its destination.
  ```go
  // A writer that fails, like a full disk or a closed connection
  type failingWriter struct{}

  func (failingWriter) Write(p []byte) (int, error) { return 0, errors.New("broken pipe") }

  func writeReport(w io.Writer) error {
  	bw := bufio.NewWriter(w)
  	fmt.Fprintln(bw, "report") // buffered: no error yet

  	// BAD: the error that matters is thrown away
  	//   defer bw.Flush()

  	// GOOD
  	if err := bw.Flush(); err != nil {
  		return fmt.Errorf("flush: %w", err)
  	}
  	return nil
  }

  func main() {
  	fmt.Println(writeReport(failingWriter{})) // flush: broken pipe
  }
  ```

- **Encoding nil slices and maps as JSON**: a nil slice encodes as `null`, not `[]`, and a nil map as `null`, not `{}`. Clients written in other languages often choke on `null` where they expect a list.
  ```go
  type Item struct{ Name string }
  type ListResponse struct {
  	Items []Item `json:"items"`
  }

  var items []Item // no items found: a nil slice

  // BAD: {"items":null} when there are no items
  bad, _ := json.Marshal(ListResponse{Items: items})

  // GOOD: {"items":[]}
  if items == nil {
  	items = []Item{}
  }
  good, _ := json.Marshal(ListResponse{Items: items})
  fmt.Println(string(bad), string(good))
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
