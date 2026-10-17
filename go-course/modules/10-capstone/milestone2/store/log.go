package store

import (
	"errors"
	"os"
	"sync"
)

// Op is the kind of mutation a Record describes.
type Op string

// The operations a log can contain.
const (
	OpSet    Op = "set"
	OpDelete Op = "del"
)

// Record is one mutation in the append-only log. On disk each record is one
// line of JSON terminated by '\n' ("JSON Lines"), for example:
//
//	{"op":"set","key":"user:1","value":"ann","exp":1735787045000000000}
//	{"op":"del","key":"user:1"}
//
// JSON escapes '\n' inside strings, so a value can never break the framing.
type Record struct {
	Op    Op     `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
	// ExpiresAt is the absolute expiry as Unix nanoseconds; 0 means never.
	ExpiresAt int64 `json:"exp,omitempty"`
}

// ErrCorrupt reports a log line that is complete (newline-terminated) but
// cannot be decoded into a valid Record. Replay wraps it with the line
// number.
var ErrCorrupt = errors.New("store: corrupt log record")

// ErrClosed is returned by operations on a closed Log or DB.
var ErrClosed = errors.New("store: closed")

// logFlags opens the log for reading (Replay) and appending. With O_APPEND
// the kernel positions every write at the current end of file, whatever the
// read offset is.
const logFlags = os.O_RDWR | os.O_CREATE | os.O_APPEND

// Log is an append-only file of Records. It is safe for concurrent use.
type Log struct {
	path string
	sync bool // fsync after every Append

	mu sync.Mutex
	f  *os.File // nil after Close
}

// OpenLog opens (creating if necessary) the log file at path. If syncWrites
// is true, every Append is followed by an fsync, so an acknowledged write
// survives power loss.
//
// Call Replay before the first Append: it is also what trims a torn final
// record left by a crash.
func OpenLog(path string, syncWrites bool) (*Log, error) {
	// TODO
	return nil, ErrClosed
}

// encodeRecord returns r as one newline-terminated JSON line.
func encodeRecord(r Record) ([]byte, error) {
	// TODO
	return nil, errors.New("not implemented")
}

// decodeRecord parses one line (without its trailing newline) and validates
// the result.
func decodeRecord(line []byte) (Record, error) {
	// TODO
	return Record{}, errors.New("not implemented")
}

// Append writes r to the end of the log. When Append returns nil the record
// has been handed to the operating system with a single write call (and, if
// the log was opened with syncWrites, flushed to stable storage).
func (l *Log) Append(r Record) error {
	// TODO
	return ErrClosed
}

// Replay reads the log from the beginning and calls fn for every record in
// order. It stops at the first error.
//
// A final line without a terminating '\n' is a torn write (the process
// died mid-Append); it was never acknowledged, so Replay discards it and
// truncates the file back to the last complete record. Any other line that
// fails to decode is corruption and yields an error wrapping ErrCorrupt.
func (l *Log) Replay(fn func(Record) error) error {
	// TODO
	return ErrClosed
}

// Compact atomically replaces the log's contents with records.
//
// It writes records to a temporary file in the same directory, fsyncs it,
// renames it over the log, and fsyncs the directory so the rename itself is
// durable. A crash at any point leaves either the complete old log or the
// complete new one, never a mixture. Appends after Compact go to the new
// file.
func (l *Log) Compact(records []Record) error {
	// TODO
	return ErrClosed
}

// writeTemp writes records to a new temporary file in dir, fsyncs and
// closes it, and returns its name. On error the file is removed.
func writeTemp(dir, pattern string, records []Record) (name string, err error) {
	// TODO
	return "", errors.New("not implemented")
}

// syncDir fsyncs a directory so that a rename inside it is durable.
func syncDir(dir string) error {
	// TODO
	return errors.New("not implemented")
}

// Close closes the log file. Further calls to Append, Replay and Compact
// return ErrClosed. Close is idempotent.
func (l *Log) Close() error {
	// TODO
	return nil
}
