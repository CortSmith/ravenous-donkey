// Package linecount counts lines in a stream of text without ever holding a
// whole line in memory.
package linecount

import "io"

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
//
// TODO: implement. A default bufio.Scanner is NOT enough: it gives up on
// lines longer than 64 KiB (bufio.ErrTooLong), and raising its limit means
// buffering whole lines. Look at bufio.Reader.ReadLine (and its isPrefix
// result) or read fixed-size chunks yourself and search them with
// bytes.IndexByte.
func Count(r io.Reader) (Stats, error) {
	return Stats{}, nil
}
