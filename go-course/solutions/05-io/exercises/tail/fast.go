package tail

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// TailSeeker returns the same lines as Tail, but for inputs that can seek
// (files, bytes.Reader, ...) it reads the input backwards from the end in
// fixed-size blocks until it has seen n line terminators, then reads only
// that suffix forwards. The cost is O(size of the last n lines) instead of
// O(size of the whole input): tailing a 10 GB log is instant.
//
// Unlike Tail it does not enforce MaxLineLength.
func TailSeeker(rs io.ReadSeeker, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}
	size, err := rs.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("tail: %w", err)
	}

	const blockSize = 64 * 1024
	buf := make([]byte, blockSize)
	end := size // we have examined bytes [end, size)
	start := int64(0)
	found := 0
	skipFinalNewline := true // a '\n' at size-1 terminates the last line; it doesn't start a new one
search:
	for end > 0 {
		blockStart := max(0, end-blockSize)
		b := buf[:end-blockStart]
		if _, err := rs.Seek(blockStart, io.SeekStart); err != nil {
			return nil, fmt.Errorf("tail: %w", err)
		}
		if _, err := io.ReadFull(rs, b); err != nil {
			return nil, fmt.Errorf("tail: %w", err)
		}
		for i := len(b) - 1; i >= 0; i-- {
			if b[i] != '\n' {
				skipFinalNewline = false
				continue
			}
			if skipFinalNewline {
				skipFinalNewline = false
				continue
			}
			found++
			if found == n {
				start = blockStart + int64(i) + 1
				break search
			}
		}
		end = blockStart
	}

	if _, err := rs.Seek(start, io.SeekStart); err != nil {
		return nil, fmt.Errorf("tail: %w", err)
	}
	var sb strings.Builder
	sb.Grow(int(size - start))
	if _, err := io.Copy(&sb, rs); err != nil {
		return nil, fmt.Errorf("tail: %w", err)
	}
	s := sb.String()
	if s == "" {
		return []string{}, nil
	}
	s = strings.TrimSuffix(s, "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines, nil
}

// TailStream is Tail for inputs that cannot seek (pipes, sockets, stdin).
// Instead of converting every line to a new string with sc.Text(), it keeps
// n reusable byte slices and copies sc.Bytes() into the oldest one, so after
// the first n lines it allocates nothing per line. Only the n survivors are
// converted to strings at the end.
func TailStream(r io.Reader, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}
	ring := make([][]byte, 0, min(n, 1024))
	next := 0
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLineLength)
	for sc.Scan() {
		if len(ring) < n {
			ring = append(ring, bytes.Clone(sc.Bytes())) // sc.Bytes() is overwritten by the next Scan
			continue
		}
		ring[next] = append(ring[next][:0], sc.Bytes()...) // reuse the slot's capacity
		next = (next + 1) % n
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("tail: %w", err)
	}
	out := make([]string, 0, len(ring))
	for i := range ring {
		out = append(out, string(ring[(next+i)%len(ring)]))
	}
	return out, nil
}
