// Package linecount counts lines in a stream of text without ever holding a
// whole line in memory.
package linecount

import (
	"bufio"
	"errors"
	"io"
)

// Stats summarises the lines of a text stream.
type Stats struct {
	// Lines is the number of lines. A line is terminated by '\n'; a final
	// line that is not terminated still counts (so "a\nb" has 2 lines, but
	// "a\n" has 1 and "" has 0).
	Lines int
	// Longest is the length in bytes of the longest line, not counting its
	// terminator. A "\r\n" terminator counts as a terminator too, so the
	// '\r' is not included either; a '\r' anywhere else is ordinary content.
	Longest int
}

// Count reads r to the end and reports its line statistics. Lines may be of
// any length: Count uses a fixed amount of memory no matter how long a line
// is. If reading fails, Count returns the error together with the
// statistics gathered so far.
func Count(r io.Reader) (Stats, error) {
	br := bufio.NewReader(r)
	var (
		s       Stats
		lineLen int  // bytes of the current line seen so far
		partial bool // we are in the middle of a line (last chunk was a prefix)
	)
	for {
		// ReadLine returns at most one buffer's worth of a line. isPrefix
		// reports that the line continues in the next call. It strips both
		// "\n" and "\r\n" for us, even when they straddle a buffer boundary.
		chunk, isPrefix, err := br.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// A long final line without '\n' arrives as prefix chunks
				// followed by a bare io.EOF: don't forget to count it.
				if partial {
					s.Lines++
					s.Longest = max(s.Longest, lineLen)
				}
				return s, nil
			}
			return s, err
		}
		lineLen += len(chunk)
		partial = isPrefix
		if isPrefix {
			continue // the line goes on; keep accumulating its length
		}
		s.Lines++
		s.Longest = max(s.Longest, lineLen)
		lineLen = 0
	}
}
