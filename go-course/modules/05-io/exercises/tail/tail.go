// Package tail returns the last lines of a stream, like the Unix tail(1)
// command, using memory proportional to the number of lines kept rather
// than to the size of the input.
package tail

import "io"

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
//
// TODO: implement with a bufio.Scanner (raise its limit with sc.Buffer) and
// a ring buffer of at most n lines: do NOT collect every line and slice off
// the end. Never return a nil slice on success. Don't trust n for an
// up-front allocation: make([]string, n) with n = 1<<62 would crash.
func Tail(r io.Reader, n int) ([]string, error) {
	return nil, nil
}
