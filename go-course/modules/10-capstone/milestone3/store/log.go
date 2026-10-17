package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	f, err := os.OpenFile(path, logFlags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	return &Log{path: path, sync: syncWrites, f: f}, nil
}

// encodeRecord returns r as one newline-terminated JSON line.
func encodeRecord(r Record) ([]byte, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// decodeRecord parses one line (without its trailing newline) and validates
// the result.
func decodeRecord(line []byte) (Record, error) {
	var r Record
	if err := json.Unmarshal(line, &r); err != nil {
		return Record{}, err
	}
	if r.Op != OpSet && r.Op != OpDelete {
		return Record{}, fmt.Errorf("unknown op %q", r.Op)
	}
	return r, nil
}

// Append writes r to the end of the log. When Append returns nil the record
// has been handed to the operating system with a single write call (and, if
// the log was opened with syncWrites, flushed to stable storage).
func (l *Log) Append(r Record) error {
	b, err := encodeRecord(r)
	if err != nil {
		return fmt.Errorf("encode record: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return ErrClosed
	}
	// One Write per record: a crash can tear the last record, but can never
	// interleave two records.
	if _, err := l.f.Write(b); err != nil {
		return fmt.Errorf("append: %w", err)
	}
	if l.sync {
		if err := l.f.Sync(); err != nil {
			return fmt.Errorf("append: sync: %w", err)
		}
	}
	return nil
}

// Replay reads the log from the beginning and calls fn for every record in
// order. It stops at the first error.
//
// A final line without a terminating '\n' is a torn write (the process
// died mid-Append); it was never acknowledged, so Replay discards it and
// truncates the file back to the last complete record. Any other line that
// fails to decode is corruption and yields an error wrapping ErrCorrupt.
func (l *Log) Replay(fn func(Record) error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return ErrClosed
	}
	if _, err := l.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("replay: %w", err)
	}

	br := bufio.NewReader(l.f)
	var good int64 // offset just past the last complete record
	for lineNo := 1; ; lineNo++ {
		line, err := br.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			if len(line) > 0 { // torn tail: drop it
				if err := l.f.Truncate(good); err != nil {
					return fmt.Errorf("replay: truncate torn record: %w", err)
				}
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("replay: %w", err)
		}

		r, err := decodeRecord(bytes.TrimSuffix(line, []byte{'\n'}))
		if err != nil {
			return fmt.Errorf("replay: line %d: %w: %v", lineNo, ErrCorrupt, err)
		}
		if err := fn(r); err != nil {
			return fmt.Errorf("replay: line %d: %w", lineNo, err)
		}
		good += int64(len(line))
	}
}

// Compact atomically replaces the log's contents with records.
//
// It writes records to a temporary file in the same directory, fsyncs it,
// renames it over the log, and fsyncs the directory so the rename itself is
// durable. A crash at any point leaves either the complete old log or the
// complete new one, never a mixture. Appends after Compact go to the new
// file.
func (l *Log) Compact(records []Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return ErrClosed
	}

	dir := filepath.Dir(l.path)
	tmpName, err := writeTemp(dir, filepath.Base(l.path)+".compact-*", records)
	if err != nil {
		return fmt.Errorf("compact: %w", err)
	}
	if err := os.Rename(tmpName, l.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("compact: %w", err)
	}
	// The new file is now the log. Everything below is about making that
	// durable and pointing l.f at it; we must not delete it on failure.
	syncErr := syncDir(dir)

	// Reopen by name with O_APPEND. Our old descriptor still refers to the
	// old, now unlinked, inode: writing to it would silently lose data.
	f, err := os.OpenFile(l.path, logFlags, 0o600)
	l.f.Close()
	l.f = f // nil on error: the Log reports ErrClosed from now on
	if err != nil {
		return fmt.Errorf("compact: reopen: %w", err)
	}
	if syncErr != nil {
		return fmt.Errorf("compact: sync dir: %w", syncErr)
	}
	return nil
}

// writeTemp writes records to a new temporary file in dir, fsyncs and
// closes it, and returns its name. On error the file is removed.
func writeTemp(dir, pattern string, records []Record) (name string, err error) {
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			tmp.Close() // may be a second Close; harmless, error ignored
			os.Remove(tmp.Name())
		}
	}()

	w := bufio.NewWriter(tmp)
	for _, r := range records {
		b, err := encodeRecord(r)
		if err != nil {
			return "", fmt.Errorf("encode: %w", err)
		}
		if _, err := w.Write(b); err != nil {
			return "", err
		}
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	if err := tmp.Sync(); err != nil { // data on disk BEFORE the rename
		return "", fmt.Errorf("sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

// syncDir fsyncs a directory so that a rename inside it is durable.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// Close closes the log file. Further calls to Append, Replay and Compact
// return ErrClosed. Close is idempotent.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}
