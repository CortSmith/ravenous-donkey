# Project: URL Shortener API

## Overview

Build a complete URL-shortening HTTP API backed by an in-memory store. The API accepts long URLs, assigns short codes, and provides redirect and statistics endpoints.

## Requirements

### Routes

The API must expose:

- **POST /links** — Create a shortened link
  - Request body: `{"url": "https://...", "code": "..."}`
    - `url` (required): the target URL, must be a valid http or https URL, max 2048 characters
    - `code` (optional): a custom short code (4-32 characters of `[A-Za-z0-9_-]`); if omitted, generate a random one
  - Response: **201 Created** with `Location` header and JSON body:
    ```json
    {
      "code": "xyz123",
      "url": "https://example.com/very/long/path",
      "short_url": "http://localhost:8080/xyz123",
      "visits": 0,
      "created_at": "2026-10-11T12:34:56Z"
    }
    ```
  - Error cases:
    - **400 Bad Request**: Invalid or missing URL, code too short/long, unknown field in JSON, or body is invalid
    - **409 Conflict**: Custom code is already in use
    - **413 Payload Too Large**: Request body exceeds 4 KiB

- **GET /{code}** — Redirect to the original URL
  - Path parameter: the short code
  - Response: **302 Found** with `Location` header pointing to the original URL
  - Increments the visit counter
  - Error case: **404 Not Found** if the code doesn't exist

- **GET /links/{code}/stats** — Fetch link statistics
  - Path parameter: the short code
  - Response: **200 OK** with the same JSON shape as POST /links
  - Error case: **404 Not Found** if the code doesn't exist

### Validation

- **URL validation**: Must be an absolute http or https URL with a non-empty host. Do not allow localhost URLs (unless testing locally).
  - Accept: `https://example.com`, `http://api.example.com/path?query=1`
  - Reject: `ftp://example.com` (wrong scheme), `/relative/path`, `example.com` (no scheme), `:8080` (no host)

- **Custom code validation**: 4-32 characters of `[A-Za-z0-9_-]` only. Case-sensitive.
  - Accept: `short`, `my_code-123`
  - Reject: `hi` (too short), `code!!!` (invalid characters), `code with spaces`

- **Request body**: Must be JSON. Unknown fields are rejected. Body size is capped at 4 KiB.

### Store Interface

The HTTP layer must not depend on a specific storage implementation. Define a `Store` interface with methods:

```go
// Checks the interface against YOUR MemoryStore (shortener/store.go).
import "gocourse/modules/08-http/project/shortener"

type Link = shortener.Link

type Store interface {
	Create(ctx context.Context, l Link) error
	Get(ctx context.Context, code string) (Link, error)
	Visit(ctx context.Context, code string) (Link, error)
}

var s Store = shortener.NewMemoryStore() // compiles: the method set matches
ctx := context.Background()
err := s.Create(ctx, Link{Code: "go", URL: "https://go.dev"})
l, err2 := s.Visit(ctx, "go")
fmt.Println(err, l.Visits, err2) // when you're done: <nil> 1 <nil>
```

- **Create**: Stores a new link. Returns `ErrExists` if the code is already taken.
- **Get**: Returns the link for a code, or `ErrNotFound`.
- **Visit**: Atomically increments the visit counter and returns the updated link, or `ErrNotFound`.

Provide a `MemoryStore` implementation for development and testing.

### Logging

Use `log/slog` to log each request with:
- **message**: "request"
- **attributes**: `method`, `path`, `status`, `duration` (as time.Duration)

Errors inside handlers should log the error details (URL validation failed, code generation exhausted) at Error level.

### Error Responses

All errors must be JSON:

```json
{"error": "error message here"}
```

The message must be safe to show the client (no internal stack traces or paths). For internal failures (code generation, store errors), log the details and send a generic message:

```json
{"error": "internal server error"}
```

## Acceptance Criteria

- [ ] Routes accept POST /links, GET /{code}, GET /links/{code}/stats with the correct methods
- [ ] URL validation rejects invalid schemes, missing hosts, and URLs longer than 2048 characters
- [ ] Custom codes are validated (4-32 characters of [A-Za-z0-9_-])
- [ ] Random codes are generated when code is omitted; collision is handled by retrying
- [ ] Responses include the correct Content-Type (application/json)
- [ ] Responses return the correct status codes (201, 302, 404, 400, 409, 413, 500)
- [ ] Location header is set on 201 and 302 responses
- [ ] Request and response bodies are JSON (validated with mime.ParseMediaType)
- [ ] Request body size is capped at 4 KiB
- [ ] Unknown fields in request JSON are rejected (DisallowUnknownFields)
- [ ] Visit counter is incremented only on redirect (GET /{code}), not on stats requests
- [ ] Every request is logged with method, path, status, and duration
- [ ] Errors are logged with details; clients see generic messages
- [ ] All tests use httptest.NewServer (no real network ports)
- [ ] Tests must pass with `go test -race ./...`

## Testing

Write comprehensive tests covering:

- Creating links with valid and invalid URLs
- Creating links with and without custom codes
- Handling code collisions and retries
- Redirecting with visit counter increment
- Fetching statistics
- Invalid path parameters
- Malformed JSON requests
- Request body size validation
- Unknown fields rejection
- Content-Type validation
- Concurrent requests (test with -race)

**Test command:**
```sh
go test ./modules/08-http/project/... -race
```

## Stretch Goals (optional)

- Add a **DELETE /{code}** endpoint to remove a link
- Implement link expiration (TTL) with a background cleanup goroutine
- Add a **PUT /links/{code}** endpoint to update a link's URL
- Return different status codes for different error types (e.g., 422 Unprocessable Entity for validation)
- Implement a real database backend (SQLite, PostgreSQL) as an alternative Store

## Tips

- Use `http.NewServeMux` with Go 1.22 patterns: `POST /links`, `GET /{code}`.
- Encode responses into a `bytes.Buffer` first, so if encoding fails the client gets a 500, not a partial response.
- Use `mime.ParseMediaType` to validate Content-Type (it handles charset parameters).
- Use `http.MaxBytesReader` to cap the request body size.
- Call `json.Decoder.DisallowUnknownFields()` to catch typos and API versioning issues.
- Drain response bodies with `io.Copy(io.Discard, resp.Body)` so connections are reused (if you implement a redirect without following it).
- Every request should pass through `r.Context()`, so store cancellations and timeouts work correctly.
- Test with `go test -race ./...` to catch data races.
