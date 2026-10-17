// Package logging practises composition with embedding: promoting methods
// from an embedded type, overriding some of them, and delegating to the
// embedded value explicitly.
package logging

import "io"

// Logger writes prefixed lines to Out. A nil *Logger, or a Logger with a
// nil Out, is valid and discards everything, so optional logging needs no
// nil checks at call sites.
type Logger struct {
	Prefix string
	Out    io.Writer
}

// Logf formats a message like fmt.Sprintf, prepends l.Prefix, appends a
// newline unless the message already ends with one, and writes the whole
// line to l.Out in a single Write call (so concurrent writers to the same
// destination never interleave within a line). Write errors are ignored, as
// with the standard library's log package.
func (l *Logger) Logf(format string, args ...any) {
	// TODO
}

// CountingWriter is an io.Writer that forwards to the embedded Writer while
// counting the bytes and newline characters that were actually written.
type CountingWriter struct {
	io.Writer
	Bytes int64
	Lines int
}

// Write writes p to the underlying writer and updates the counters using
// only the n bytes that were written, even when an error is returned.
func (c *CountingWriter) Write(p []byte) (int, error) {
	// TODO: count. (Careful: what would `c.Write(p)` call here?)
	return c.Writer.Write(p)
}

// Store is a minimal string key-value store.
type Store interface {
	Get(key string) (string, bool)
	Set(key, value string)
	Delete(key string)
}

// MapStore is a Store backed by a map. Its zero value is ready to use.
// (Provided complete; nothing to do here.)
type MapStore struct {
	m map[string]string
}

var _ Store = (*MapStore)(nil)

// Get returns the value for key and whether it was present.
func (s *MapStore) Get(key string) (string, bool) {
	v, ok := s.m[key]
	return v, ok
}

// Set stores value under key.
func (s *MapStore) Set(key, value string) {
	if s.m == nil {
		s.m = make(map[string]string)
	}
	s.m[key] = value
}

// Delete removes key. Deleting a missing key is a no-op.
func (s *MapStore) Delete(key string) { delete(s.m, key) }

// AuditedStore decorates another Store: Get is promoted unchanged from the
// embedded Store, while Set and Delete are overridden to log the mutation
// before delegating to the embedded Store.
type AuditedStore struct {
	Store
	Log *Logger
}

// NewAuditedStore returns an AuditedStore that wraps s and logs to log
// (which may be nil to disable logging).
func NewAuditedStore(s Store, log *Logger) *AuditedStore {
	return &AuditedStore{Store: s} // TODO: something is missing
}

// Set logs "set <key>=<value>" and then stores the value in the wrapped
// Store.
func (a *AuditedStore) Set(key, value string) {
	// TODO: log first.
	a.Store.Set(key, value)
}

// Delete logs "delete <key>" and then deletes the key from the wrapped
// Store.
func (a *AuditedStore) Delete(key string) {
	// TODO: log first.
	a.Store.Delete(key)
}

// Note: there is deliberately no Get method here. Don't add one.
