package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// LoggingFast behaves like Logging but avoids most per-request allocations:
//   - it skips all work when the logger is not enabled for Info;
//   - it uses LogAttrs with typed slog.Attr values, so ints and durations are
//     not boxed into interfaces as they are with the key/value ...any form;
//   - attrs live in a fixed-size array on the stack instead of a growing slice.
func LoggingFast(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if !logger.Enabled(ctx, slog.LevelInfo) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)

			var buf [6]slog.Attr
			attrs := append(buf[:0],
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.statusCode()),
				slog.Int("bytes", sw.bytes),
				slog.Duration("duration", time.Since(start)),
			)
			if id, ok := RequestIDFrom(ctx); ok {
				attrs = append(attrs, slog.String("request_id", id))
			}
			logger.LogAttrs(ctx, slog.LevelInfo, "request", attrs...)
		})
	}
}
