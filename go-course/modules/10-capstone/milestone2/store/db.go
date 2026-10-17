package store

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Options configures a DB. The zero value is valid: real clock, no fsync.
type Options struct {
	// Now returns the current time. nil means time.Now.
	Now func() time.Time
	// Sync makes every write fsync the log before returning.
	Sync bool
}

// DB is a durable key-value store: every mutation is appended to a log
// before it is applied in memory, and the log is replayed on Open.
//
// Reads (Get, Keys) are served from memory and never touch the disk.
// Writes (Set, Delete, Compact) are serialized so the order of records in
// the log always matches the order in which they were applied.
type DB struct {
	now func() time.Time
	mem *Store

	mu     sync.Mutex // serializes writers; guards log and closed
	log    *Log
	closed bool
}

// Open opens the database stored at path, creating it if it does not exist,
// and rebuilds the in-memory state by replaying the log.
func Open(path string, opts Options) (*DB, error) {
	// TODO
	return nil, ErrClosed
}

// apply applies one replayed record to the in-memory store.
func (db *DB) apply(r Record) error {
	// TODO
	return errors.New("not implemented")
}

// toUnixNano converts an expiry to its log form; zero stays zero.
func toUnixNano(t time.Time) int64 {
	// TODO
	return 0
}

// fromUnixNano is the inverse of toUnixNano.
func fromUnixNano(n int64) time.Time {
	// TODO
	return time.Time{}
}

// Get returns the value stored under key, or ErrNotFound.
func (db *DB) Get(key string) (string, error) {
	// TODO
	return "", ErrNotFound
}

// Keys returns the live keys in ascending order.
func (db *DB) Keys() []string {
	// TODO
	return nil
}

// Set durably stores value under key. If ttl > 0 the key expires ttl from
// now; the expiry is persisted as an absolute time, so restarting the
// process does not extend it.
func (db *DB) Set(key, value string, ttl time.Duration) error {
	// TODO
	return ErrClosed
}

// Delete durably removes key and reports whether a live entry existed.
// Deleting an absent key writes nothing to the log.
func (db *DB) Delete(key string) (bool, error) {
	// TODO
	return false, ErrClosed
}

// Compact rewrites the log so it holds exactly one set record per live key.
// Deleted, overwritten and expired entries disappear from disk.
func (db *DB) Compact() error {
	// TODO
	return ErrClosed
}

// RunJanitor removes expired entries from memory every interval until ctx
// is canceled. See Store.RunJanitor.
func (db *DB) RunJanitor(ctx context.Context, interval time.Duration) {
	// TODO
}

// Close closes the log. Writes after Close return ErrClosed; Get and Keys
// keep serving the in-memory data. Close is idempotent.
func (db *DB) Close() error {
	// TODO
	return nil
}
