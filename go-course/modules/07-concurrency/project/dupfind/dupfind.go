// Package dupfind finds files with identical contents under a directory
// tree. It groups files by size first (cheap: one stat per file) and only
// hashes files that share a size with at least one other file, using a
// bounded pool of hashing goroutines.
package dupfind

import (
	"context"
	"fmt"
	"io"
)

// HashFunc returns a content hash for the file at path.
type HashFunc func(ctx context.Context, path string) (string, error)

// Options configures Find. The zero value is valid.
type Options struct {
	// Workers is the maximum number of files hashed concurrently.
	// Values < 1 mean runtime.GOMAXPROCS(0).
	Workers int
	// MinSize skips files smaller than MinSize bytes. Empty files are
	// always skipped, whatever MinSize is.
	MinSize int64
	// Hash computes file hashes. nil means HashFile.
	Hash HashFunc
}

// Group is a set of two or more files with identical size and hash.
type Group struct {
	Size  int64    // size of each file in bytes
	Hash  string   // hex-encoded hash shared by every file
	Paths []string // sorted, at least two
}

// HashFile returns the hex-encoded SHA-256 of the file at path.
func HashFile(ctx context.Context, path string) (string, error) {
	// TODO: implement
	return "", fmt.Errorf("not implemented")
}

// Find walks root and returns every group of duplicate regular files.
// Symlinks and other non-regular files are ignored. Groups are sorted by
// Size descending, then by their first path; paths within a group are
// sorted. Find returns a nil slice (and nil error) when there are no
// duplicates.
//
// If walking or hashing fails, or ctx is canceled, Find stops hashing,
// waits for its goroutines to exit, and returns the error.
func Find(ctx context.Context, root string, opts Options) ([]Group, error) {
	// TODO: implement
	return nil, fmt.Errorf("not implemented")
}

// filesBySize walks root and groups regular files of at least minSize bytes
// by size.
func filesBySize(ctx context.Context, root string, minSize int64) (map[int64][]string, error) {
	// TODO: implement
	return nil, fmt.Errorf("not implemented")
}

// hashAll hashes paths with a bounded worker pool and returns the hashes in
// the same order as paths. It stops at the first error.
func hashAll(ctx context.Context, paths []string, opts Options) ([]string, error) {
	// TODO: implement
	return nil, fmt.Errorf("not implemented")
}

// Wasted returns the number of bytes that could be reclaimed by keeping one
// copy from each group: the sum of Size * (len(Paths) - 1).
func Wasted(groups []Group) int64 {
	// TODO: implement
	return 0
}

// WriteReport writes a human-readable report of groups to w. See the
// project README for the exact format.
func WriteReport(w io.Writer, groups []Group) error {
	// TODO: implement
	return fmt.Errorf("not implemented")
}
