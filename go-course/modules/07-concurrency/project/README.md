# Project: Duplicate File Finder

## Goal

Build a command-line tool `dupfind` that walks a directory tree, groups files by size, hashes files that share a size, and reports duplicate files (files with identical contents). Use a bounded worker pool to hash files concurrently.

## Requirements

1. **Core API** (package `dupfind`):
   - `Find(ctx, root, opts) ([]Group, error)`: walks the directory tree starting at `root` and returns groups of duplicate files.
   - `Group`: a set of two or more files with the same size and content hash.
   - `HashFile(ctx, path)`: compute the SHA-256 hash of a file (provided as a stub; implement it).
   - `Options`: controls the hash function to use (default: `HashFile`), the number of concurrent workers (default: `runtime.GOMAXPROCS(0)`), and a minimum file size filter (empty files are always skipped).
   - `Wasted(groups)`: returns the number of bytes that could be reclaimed by keeping one copy from each group.
   - `WriteReport(w, groups)`: writes a human-readable report of duplicates.

2. **Command** (package `main` in `cmd/dupfind`):
   - Flags: `-workers N` (max concurrent hashes), `-min-size BYTES` (default 1), optional directory (default: current directory).
   - Graceful shutdown on Ctrl+C via `signal.NotifyContext`.
   - Exit codes: 0 on success, 1 on file system / hash errors, 2 on invalid flags.

3. **Concurrency**:
   - Group files by size first (free operation: `os.Stat`); only hash files in size collisions.
   - Use a bounded worker pool to hash files concurrently; respect the `-workers` limit.
   - Stop hashing and clean up gracefully on errors or context cancellation.
   - No goroutine leaks; test with `runtime.NumGoroutine()`.

4. **Testing**:
   - `go test -race ./modules/07-concurrency/project/... -timeout 60s`
   - Tests use `t.TempDir()` to create file trees.
   - Verify correct grouping (size, hash, order), handling of empty files, `MinSize`, custom hash functions.
   - Verify concurrency bounds and no leaks.

## Report Format

The report written by `WriteReport` is human-readable, one group per section:

```
group N: M files, S bytes each, hash HHHHHHHH
  /path/to/file1
  /path/to/file2
  ...
G groups, B bytes reclaimable
```

- Group number starts at 1.
- The hash is truncated to 12 hex characters (or the full hash if shorter).
- File paths are printed as-is (not quoted or escaped).
- The final line summarizes total groups and wasted bytes.
- If there are no duplicates, output is just `0 groups, 0 bytes reclaimable\n`.

## Acceptance Criteria

- All tests pass with `go test -race ./modules/07-concurrency/project/...`.
- Code is `gofmt`-clean and passes `go vet`.
- The tool correctly identifies duplicates by size and content hash.
- Concurrency is bounded: peak active hashes ≤ `-workers`.
- Graceful cancellation: test with `-race` confirms no leaks.

## Stretch Goals

- Parallel hashing of very large files (use `io.Reader` chunks).
- Estimate time remaining by bytes hashed vs. total bytes to hash.
- Deduplicate by hardlinking (be careful with test setup).
