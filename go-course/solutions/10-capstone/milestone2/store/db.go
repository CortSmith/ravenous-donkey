package store

import (
	"context"
	"fmt"
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
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	l, err := OpenLog(path, opts.Sync)
	if err != nil {
		return nil, err
	}
	db := &DB{now: now, mem: New(WithClock(now)), log: l}
	if err := l.Replay(db.apply); err != nil {
		l.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

// apply applies one replayed record to the in-memory store.
func (db *DB) apply(r Record) error {
	switch r.Op {
	case OpSet:
		// An already-expired record is still applied: it must hide any
		// older value of the same key. Reads treat it as absent and the
		// janitor or the next Compact removes it.
		db.mem.SetExpiresAt(r.Key, r.Value, fromUnixNano(r.ExpiresAt))
	case OpDelete:
		db.mem.Delete(r.Key)
	default:
		return fmt.Errorf("unknown op %q", r.Op) // decodeRecord already rejects this
	}
	return nil
}

// toUnixNano converts an expiry to its log form; zero stays zero.
func toUnixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

// fromUnixNano is the inverse of toUnixNano.
func fromUnixNano(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// Get returns the value stored under key, or ErrNotFound.
func (db *DB) Get(key string) (string, error) {
	return db.mem.Get(key)
}

// Keys returns the live keys in ascending order.
func (db *DB) Keys() []string {
	return db.mem.Keys()
}

// Set durably stores value under key. If ttl > 0 the key expires ttl from
// now; the expiry is persisted as an absolute time, so restarting the
// process does not extend it.
func (db *DB) Set(key, value string, ttl time.Duration) error {
	var expiresAt time.Time
	if ttl > 0 {
		// Round-trip through the log representation so memory and disk agree
		// to the nanosecond (and the monotonic clock reading is dropped).
		expiresAt = fromUnixNano(db.now().Add(ttl).UnixNano())
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return ErrClosed
	}
	// Write-ahead: log first, then apply. If the append fails, memory is
	// unchanged and the caller sees the error.
	rec := Record{Op: OpSet, Key: key, Value: value, ExpiresAt: toUnixNano(expiresAt)}
	if err := db.log.Append(rec); err != nil {
		return fmt.Errorf("set %q: %w", key, err)
	}
	db.mem.SetExpiresAt(key, value, expiresAt)
	return nil
}

// Delete durably removes key and reports whether a live entry existed.
// Deleting an absent key writes nothing to the log.
func (db *DB) Delete(key string) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return false, ErrClosed
	}
	// Writers are serialized by db.mu, so nothing can create key between
	// this check and the append below.
	if _, err := db.mem.Get(key); err != nil {
		return false, nil
	}
	if err := db.log.Append(Record{Op: OpDelete, Key: key}); err != nil {
		return false, fmt.Errorf("delete %q: %w", key, err)
	}
	return db.mem.Delete(key), nil
}

// Compact rewrites the log so it holds exactly one set record per live key.
// Deleted, overwritten and expired entries disappear from disk.
func (db *DB) Compact() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return ErrClosed
	}
	// Holding db.mu means no write can slip in between taking the snapshot
	// and swapping the file, which would lose that write.
	items := db.mem.Items()
	recs := make([]Record, len(items))
	for i, it := range items {
		recs[i] = Record{Op: OpSet, Key: it.Key, Value: it.Value, ExpiresAt: toUnixNano(it.ExpiresAt)}
	}
	return db.log.Compact(recs)
}

// RunJanitor removes expired entries from memory every interval until ctx
// is canceled. See Store.RunJanitor.
func (db *DB) RunJanitor(ctx context.Context, interval time.Duration) {
	db.mem.RunJanitor(ctx, interval)
}

// Close closes the log. Writes after Close return ErrClosed; Get and Keys
// keep serving the in-memory data. Close is idempotent.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return nil
	}
	db.closed = true
	return db.log.Close()
}
