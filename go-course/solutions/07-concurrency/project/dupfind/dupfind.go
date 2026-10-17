// Package dupfind finds files with identical contents under a directory
// tree. It groups files by size first (cheap: one stat per file) and only
// hashes files that share a size with at least one other file, using a
// bounded pool of hashing goroutines.
package dupfind

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
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
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() // read-only: a Close error cannot lose data

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
	bySize, err := filesBySize(ctx, root, max(opts.MinSize, 1))
	if err != nil {
		return nil, err
	}

	var candidates []string
	sizeOf := make(map[string]int64)
	for size, paths := range bySize {
		if len(paths) < 2 {
			continue // a unique size cannot have a duplicate: never read it
		}
		for _, p := range paths {
			candidates = append(candidates, p)
			sizeOf[p] = size
		}
	}

	hashes, err := hashAll(ctx, candidates, opts)
	if err != nil {
		return nil, err
	}

	type key struct {
		size int64
		hash string
	}
	byKey := make(map[key][]string)
	for i, p := range candidates {
		k := key{sizeOf[p], hashes[i]}
		byKey[k] = append(byKey[k], p)
	}

	var groups []Group
	for k, paths := range byKey {
		if len(paths) < 2 {
			continue
		}
		slices.Sort(paths)
		groups = append(groups, Group{Size: k.size, Hash: k.hash, Paths: paths})
	}
	slices.SortFunc(groups, func(a, b Group) int {
		return cmp.Or(
			cmp.Compare(b.Size, a.Size), // larger files first
			cmp.Compare(a.Paths[0], b.Paths[0]),
		)
	})
	return groups, nil
}

// filesBySize walks root and groups regular files of at least minSize bytes
// by size.
func filesBySize(ctx context.Context, root string, minSize int64) (map[int64][]string, error) {
	bySize := make(map[int64][]string)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil // directories, symlinks, devices, sockets...
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() >= minSize {
			bySize[info.Size()] = append(bySize[info.Size()], path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	return bySize, nil
}

// hashAll hashes paths with a bounded worker pool and returns the hashes in
// the same order as paths. It stops at the first error.
func hashAll(ctx context.Context, paths []string, opts Options) ([]string, error) {
	hash := opts.Hash
	if hash == nil {
		hash = HashFile
	}
	workers := opts.Workers
	if workers < 1 {
		workers = runtime.GOMAXPROCS(0)
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	hashes := make([]string, len(paths))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(workers, len(paths)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				h, err := hash(ctx, paths[i])
				if err != nil {
					cancel(fmt.Errorf("hash %s: %w", paths[i], err))
					return
				}
				hashes[i] = h
			}
		}()
	}

feed:
	for i := range paths {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	return hashes, nil
}

// Wasted returns the number of bytes that could be reclaimed by keeping one
// copy from each group: the sum of Size * (len(Paths) - 1).
func Wasted(groups []Group) int64 {
	var n int64
	for _, g := range groups {
		n += g.Size * int64(len(g.Paths)-1)
	}
	return n
}

// WriteReport writes a human-readable report of groups to w. See the
// project README for the exact format.
func WriteReport(w io.Writer, groups []Group) error {
	for i, g := range groups {
		short := g.Hash[:min(12, len(g.Hash))]
		if _, err := fmt.Fprintf(w, "group %d: %d files, %d bytes each, hash %s\n", i+1, len(g.Paths), g.Size, short); err != nil {
			return err
		}
		for _, p := range g.Paths {
			if _, err := fmt.Fprintf(w, "  %s\n", p); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(w, "%d groups, %d bytes reclaimable\n", len(groups), Wasted(groups))
	return err
}
