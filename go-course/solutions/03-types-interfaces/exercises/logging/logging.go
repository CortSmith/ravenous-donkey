// Package logging practises composition with embedding: promoting methods
// from an embedded type, overriding some of them, and delegating to the
// embedded value explicitly.
package logging

import (
	"bytes"
	"fmt"
	"io"
)

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
	if l == nil || l.Out == nil {
		return
	}
	line := l.Prefix + fmt.Sprintf(format, args...)
	if len(line) == 0 || line[len(line)-1] != '\n' {
		line += "\n"
	}
	_, _ = io.WriteString(l.Out, line)
}

// CountingWriter is an io.Writer that forwards to the embedded Writer while
// counting the bytes and newline characters that were actually written.
//
// Embedding io.Writer means CountingWriter would satisfy io.Writer even
// without its own Write method; defining Write here overrides (shadows) the
// promoted one, and c.Writer.Write reaches the embedded value explicitly.
type CountingWriter struct {
	io.Writer
	Bytes int64
	Lines int
}

// Write writes p to the underlying writer and updates the counters using
// only the n bytes that were written, even when an error is returned.
func (c *CountingWriter) Write(p []byte) (int, error) {
	n, err := c.Writer.Write(p)
	c.Bytes += int64(n)
	c.Lines += bytes.Count(p[:n], []byte{'\n'})
	return n, err
}

// Store is a minimal string key-value store.
type Store interface {
	Get(key string) (string, bool)
	Set(key, value string)
	Delete(key string)
}

// MapStore is a Store backed by a map. Its zero value is ready to use.
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

var _ Store = (*AuditedStore)(nil)

// NewAuditedStore returns an AuditedStore that wraps s and logs to log
// (which may be nil to disable logging).
func NewAuditedStore(s Store, log *Logger) *AuditedStore {
	return &AuditedStore{Store: s, Log: log}
}

// Set logs "set <key>=<value>" and then stores the value in the wrapped
// Store.
func (a *AuditedStore) Set(key, value string) {
	a.Log.Logf("set %s=%s", key, value)
	a.Store.Set(key, value)
}

// Delete logs "delete <key>" and then deletes the key from the wrapped
// Store.
func (a *AuditedStore) Delete(key string) {
	a.Log.Logf("delete %s", key)
	a.Store.Delete(key)
}
