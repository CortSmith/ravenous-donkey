// Package tail returns the last lines of a stream, like the Unix tail(1)
// command, using memory proportional to the number of lines kept rather
// than to the size of the input.
package tail

import (
	"bufio"
	"fmt"
	"io"
)

// MaxLineLength is the longest line, in bytes, that Tail accepts.
const MaxLineLength = 1 << 20 // 1 MiB

// Tail reads r to the end and returns its last n lines, oldest first,
// without their "\n" or "\r\n" terminators. A final line without a
// terminator is still a line. If r has fewer than n lines, all of them are
// returned. If n <= 0, Tail returns an empty slice without reading r.
//
// A line longer than MaxLineLength makes Tail fail with an error wrapping
// bufio.ErrTooLong. Read errors are returned (wrapped) too; in both cases
// the returned slice is nil.
func Tail(r io.Reader, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}

	// ring holds the most recent lines. Once full, next points at the
	// oldest line, which is the one to overwrite.
	ring := make([]string, 0, min(n, 1024)) // don't trust n for a huge up-front allocation
	next := 0

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLineLength)
	for sc.Scan() {
		if len(ring) < n {
			ring = append(ring, sc.Text())
			continue
		}
		ring[next] = sc.Text()
		next = (next + 1) % n
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("tail: %w", err)
	}

	// Unroll the ring: oldest is at next, wrapping around to next-1.
	// (When the ring never filled up, next is 0 and this is a plain copy.)
	out := make([]string, 0, len(ring))
	out = append(out, ring[next:]...)
	out = append(out, ring[:next]...)
	return out, nil
}
