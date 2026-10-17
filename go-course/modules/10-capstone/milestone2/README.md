# Milestone 2: Persistence via Append-Only Log

**Time budget:** ≤ 60 minutes

> **Provided for you:** `store/memory.go` is a complete reference implementation of Milestone 1 (with `SetExpiresAt`/`Items` helpers added for replay), so this milestone is independent of your M1 code. You implement `store/log.go` and `store/db.go`. Feel free to swap in your own M1 store instead.

## Overview

Make the store durable: every mutation (Set or Delete) is appended to an append-only log before it is applied in memory. On startup, the log is replayed to rebuild the exact state. Handle crashes gracefully (torn writes, recovery). Optionally, compact the log to shrink it.

## Specification

### API

This milestone extends Milestone 1's Store with two new methods and introduces Log and DB.

```go
// The API of milestone2/store. Each assertion below checks YOUR signature:
// this block stops compiling if one of them differs.
import "gocourse/modules/10-capstone/milestone2/store"

var (
	// Store additions (from Milestone 1)

	// SetExpiresAt stores value under key with an absolute expiry time.
	// A zero expiresAt means the entry never expires.
	// An expiresAt in the past stores an already-expired entry (reads treat as absent).
	//
	// Persistence needs this: a TTL is relative to "now", but the log records
	// the absolute expiry so a restart doesn't extend the key's life.
	_ func(s *store.Store, key, value string, expiresAt time.Time) = (*store.Store).SetExpiresAt

	// Items returns a snapshot of every live entry, sorted by key.
	// Each Item has Key, Value, and ExpiresAt (zero = never expires).
	_ func(s *store.Store) []store.Item = (*store.Store).Items

	// Log is an append-only file of Records.

	// OpenLog opens the log file at path, creating if necessary.
	// If syncWrites is true, every Append fsyncs (survives power loss).
	// Call Replay before the first Append.
	_ func(path string, syncWrites bool) (*store.Log, error) = store.OpenLog

	// Append writes r to the end of the log.
	_ func(l *store.Log, r store.Record) error = (*store.Log).Append

	// Replay reads the log from the beginning, calling fn for each record.
	// Handles torn final records (incomplete write from a crash).
	_ func(l *store.Log, fn func(store.Record) error) error = (*store.Log).Replay

	// Compact atomically replaces the log with a snapshot of the given records.
	// Removes deleted, overwritten, and expired entries from disk.
	_ func(l *store.Log, records []store.Record) error = (*store.Log).Compact

	// Close closes the log file.
	_ func(l *store.Log) error = (*store.Log).Close

	// DB is a durable key-value store (wraps Store + Log).

	// Open opens the database at path, replaying the log to rebuild state.
	_ func(path string, opts store.Options) (*store.DB, error) = store.Open

	// Get, Keys, Delete, RunJanitor same as Store.

	// Set durably stores value under key. TTL is relative to now;
	// it's persisted as an absolute time.
	_ func(db *store.DB, key, value string, ttl time.Duration) error = (*store.DB).Set

	// Delete durably removes key. Returns false if the key was absent.
	_ func(db *store.DB, key string) (bool, error) = (*store.DB).Delete

	// Compact rewrites the log with one "set" record per live key.
	_ func(db *store.DB) error = (*store.DB).Compact

	// Close closes the log.
	_ func(db *store.DB) error = (*store.DB).Close
)

// The data types, already declared in the starter:
var (
	_ = store.Item{Key: "k", Value: "v", ExpiresAt: time.Time{}}
	_ = store.Record{
		Op:        store.OpSet, // or store.OpDelete
		Key:       "k",
		Value:     "v", // only for "set"
		ExpiresAt: 0,   // Unix nanoseconds; 0 = never expires
	}
	_ = store.Options{
		Now:  nil,   // nil means time.Now
		Sync: false, // fsync after every write?
	}
)

func main() {
	dir, _ := os.MkdirTemp("", "kvdb")
	defer os.RemoveAll(dir)
	db, err := store.Open(filepath.Join(dir, "kv.log"), store.Options{})
	if err != nil {
		fmt.Println("Open:", err)
		return
	}
	defer db.Close()
	fmt.Println(db.Set("lang", "go", 0)) // when you're done: <nil>
	fmt.Println(db.Get("lang"))          // go <nil>
}
```

### Behavior

**Write-Ahead Logging (WAL):**
- Every write (Set or Delete) is appended to the log before it's applied in memory.
- If the append succeeds, the write is durable.
- If the append fails, the in-memory state is unchanged and the error is reported.

**Replay:**
- On Open, the log is replayed by calling a callback for each record.
- The callback applies each record to the in-memory Store.

**Torn Writes:**
- If the process dies mid-write, the last line may lack a terminating `\n`.
- Replay detects this (EOF before `\n`) and truncates the file, discarding the incomplete record.
- The incomplete record was never acknowledged to the caller, so losing it is safe.

**Compaction:**
- Compact takes a snapshot of live items, writes them to a temp file, fsyncs, renames atomically over the log, and fsyncs the directory.
- A crash at any point leaves either the complete old log or the complete new one, never a mixture.

**Concurrency:**
- All operations are safe for concurrent use.
- Writes (Set, Delete, Compact) are serialized via a mutex so the log order is deterministic.
- Reads (Get, Keys) are served from the in-memory Store and don't touch the disk.

### Testing

```bash
cd /home/csmith/Workspace/tools/go-course
go test ./modules/10-capstone/milestone2/... -race
```

The test files are provided (db_test.go, log_test.go). They verify:
- Opening and replaying a log.
- Handling torn writes (truncation on recovery).
- Compaction shrinking the log.
- Durability: data survives a process restart.
- Concurrency and `-race` safety.

---

## Implementation Guide

### Design: Store Modifications

The starter Store is your Milestone 1 solution. Add:

1. **SetExpiresAt**: Like Set, but takes an absolute `expiresAt` time instead of a duration.

2. **Items**: Return a slice of Item (struct with Key, Value, ExpiresAt), filtered to live entries, sorted by key.

### Design: Log

Records are stored as **JSON Lines** (one JSON object per line):

```json
{"op":"set","key":"user:1","value":"alice","exp":1735787045000000000}
{"op":"set","key":"user:2","value":"bob","exp":0}
{"op":"del","key":"user:1"}
```

**Encoding/Decoding:**
- Use `encoding/json` to marshal/unmarshal Record.
- Append a `\n` after each record (the line terminator).
- Strip the `\n` when parsing.

**OpenLog:**
- Use `os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)` to open for reading and appending.
- The O_APPEND flag ensures the kernel positions every write at the end, regardless of the read offset.

**Append:**
- Encode the record to JSON, append a newline, and write in one operation.
- If `l.sync` is true, call `f.Sync()` to ensure the write reaches disk.
- Errors in writing or syncing should be wrapped and returned.

**Replay:**
- Seek to the beginning of the file.
- Use `bufio.Reader` and `ReadBytes('\n')` to read lines.
- If EOF is reached with unread data, that's a torn record: truncate the file to the last complete line.
- For each complete line, decode and call the provided callback.
- If decoding fails, return an error wrapping ErrCorrupt.

**Compact:**
- Write records to a temp file in the same directory (use `os.CreateTemp`).
- Fsync the temp file.
- Close the temp file.
- Rename the temp file over the old log (atomic, but not durable yet).
- Fsync the directory to make the rename durable.
- Reopen the log with O_APPEND (your old file descriptor points to the unlinked inode; writes would be lost).

### Design: DB

The DB wraps a Store and a Log:

1. **Open**: Creates a Log, replays it into a new Store, and returns the DB.

2. **apply**: Called by Replay for each record. Updates the Store based on the operation.

3. **Set**: Lock, write to log, unlock, apply in memory. If the log write fails, don't apply.

4. **Delete**: Lock, check if the key is live, write to log if it is, unlock, apply in memory. If the key is absent, don't write to the log.

5. **Compact**: Lock, take a snapshot (Items), write it to the log via Compact, unlock.

**Time conversions:**
- `toUnixNano(t time.Time) int64`: Converts an absolute time to Unix nanoseconds (used for storage).
- `fromUnixNano(n int64) time.Time`: Converts back.
- A zero time (never expires) is stored as 0 and converted back to zero.

---

## Common Mistakes

1. **Forgetting to fsync**: If you don't fsync after a write, a power loss can lose data. Even worse, a `Sync() bool` flag that's ignored.

2. **Not truncating torn records**: If you don't truncate the file after detecting a torn tail, subsequent Replays will read garbage.

3. **Not reopening the log after Compact**: After renaming the temp file, your old file descriptor points to the now-unlinked inode. Writes go nowhere. You must reopen with O_APPEND.

4. **Not fsyncing the directory**: After renaming a file, fsync the directory so the rename itself is durable, not just the file contents.

5. **Deleting during replay**: If Delete is called during Replay, it must not write to the log (you're replaying, not accepting new writes). Let the DB's apply method call SetExpiresAt directly, not Set.

6. **Not serializing writes**: If two goroutines call Set concurrently, they might interleave their log writes, corrupting the order. Use a mutex around log operations.

7. **Panic on encode errors**: If JSON encoding fails (very rare but possible), handle it gracefully; don't panic.

---

## Optimization Notes

- **Batch compactions**: If the log grows large, consider compacting after N deletions or M log lines. (Not required for this milestone, but good to know.)
- **Incremental log**: Instead of rewriting the entire log on every Compact, you could append a "checkpoint" record. (Advanced; not required.)

## Stretch Goals (Optional)

1. **Measure the cost of durability.** Write `BenchmarkAppend` with two
   sub-benchmarks, `OpenLog(path, false)` and `OpenLog(path, true)`, using
   `b.TempDir()`. How many times slower is an fsynced append on your
   machine? What does that mean for how many writes per second `kvd` can
   acknowledge?
2. **Group commit.** Let concurrent `Append` calls share one `fsync`: writers
   enqueue their record and wait; a single goroutine writes everything
   queued, fsyncs once, then wakes all the writers. Prove it keeps the
   durability guarantee with a test, and measure the throughput with
   `b.RunParallel`.
3. **Auto-compaction.** Compact automatically when the log has more than
   twice as many records as there are live keys.

---

## Rubric (for instructor review)

When reviewing a student's solution, check:

- ✓ All functions compile and tests pass with `-race`.
- ✓ `Open` replays the log correctly into a fresh Store.
- ✓ `Set` and `Delete` write to the log before applying in memory.
- ✓ Torn writes are detected and the log is truncated.
- ✓ `Compact` writes to a temp file, fsyncs, renames, and fsyncs the directory.
- ✓ After `Compact`, the log is reopened with O_APPEND (new writes go to the new file).
- ✓ Data survives a process restart (Open replays correctly).
- ✓ Writes are serialized; the log order is deterministic.
- ✓ `-race` detector finds no issues.
- ✓ No sleeps in the code.

---

## Advanced: Fsync Semantics

(Optional deeper dive)

**What does fsync actually do?**
- `fsync()` flushes the file's dirty pages to stable storage (usually the SSD's NAND or HDD's platter).
- Without fsync, the OS buffer cache might hold data for seconds before writing.
- A power loss before fsync = data loss.

**Durability guarantees:**
- After `f.Write()` + `f.Sync()` returns nil, the data is on disk.
- After `os.Rename()` (atomic at the filesystem level), the rename is in the journal, but not persisted.
- After `dir.Sync()`, the rename is on disk.

**Why Compact fsyncs the temp file before renaming:**
- If the process dies during the rename, we're safe.
- The old log is still there (rename is atomic but not yet durable).
- On recovery, we see the old log and replay it (losing the compaction).
- The temp file is lost (fsync'd but never renamed), so no data loss.

**Why Compact fsyncs the directory:**
- If the process dies after the rename, we're safe.
- The new log is on disk.
- On recovery, we see the new (compacted) log and replay it.

---

## Next Steps

When all tests pass, you're ready for **Milestone 3: HTTP API**.
