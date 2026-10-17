// Package middleware provides composable net/http middleware: request IDs
// carried in the request context, structured access logging with log/slog,
// and panic recovery.
package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Middleware wraps an http.Handler with extra behaviour.
type Middleware func(http.Handler) http.Handler

// Chain wraps h so that mws[0] is the outermost middleware: a request passes
// through mws[0], then mws[1], ..., then h. Chain(h) returns h unchanged.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	// Wrap from the inside out so the first middleware ends up outermost.
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// RequestIDHeader is the header used to receive and return request IDs.
const RequestIDHeader = "X-Request-ID"

// maxRequestIDLen bounds IDs accepted from clients so a caller cannot make us
// log arbitrarily large strings.
const maxRequestIDLen = 64

// requestIDKey is unexported so no other package can read or overwrite the
// value by accident: context keys are compared by type and value.
type requestIDKey struct{}

// RequestIDFrom returns the request ID stored by the RequestID middleware.
func RequestIDFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey{}).(string)
	return id, ok
}

// RequestID assigns every request an ID. An incoming X-Request-ID header is
// reused when it is non-empty and at most 64 bytes; otherwise newID is called.
// The ID is stored in the request context (see RequestIDFrom) and echoed in
// the X-Request-ID response header.
func RequestID(newID func() string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(RequestIDHeader)
			if id == "" || len(id) > maxRequestIDLen {
				id = newID()
			}
			w.Header().Set(RequestIDHeader, id)
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Logging logs one Info record with message "request" per request, with the
// attributes method, path, status, bytes, duration and, when present,
// request_id.
func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)

			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.statusCode(),
				"bytes", sw.bytes,
				"duration", time.Since(start),
			}
			if id, ok := RequestIDFrom(r.Context()); ok {
				attrs = append(attrs, "request_id", id)
			}
			logger.InfoContext(r.Context(), "request", attrs...)
		})
	}
}

// Recover turns a panic in next into a 500 response (if nothing has been
// written yet) and logs it at Error level with message "panic recovered" and
// a "panic" attribute. http.ErrAbortHandler is re-panicked: it is the
// documented way for a handler to abort a response, and net/http handles it.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w}
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if v == http.ErrAbortHandler {
					panic(v)
				}
				logger.ErrorContext(r.Context(), "panic recovered",
					"panic", fmt.Sprint(v),
					"method", r.Method,
					"path", r.URL.Path,
				)
				// If the handler already started the response, the status
				// line is on the wire; writing a 500 now would only produce
				// a "superfluous WriteHeader" warning and a corrupt body.
				if !sw.wroteHeader {
					http.Error(sw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(sw, r)
		})
	}
}

// statusWriter records the status code and byte count of a response.
type statusWriter struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		// An implicit 200, exactly as net/http does.
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

// statusCode reports the status sent; a handler that wrote nothing at all
// produces an implicit 200.
func (w *statusWriter) statusCode() int {
	if !w.wroteHeader {
		return http.StatusOK
	}
	return w.status
}

// Unwrap lets http.ResponseController reach the underlying writer, so
// Flush, Hijack and SetWriteDeadline keep working through the wrapper.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
