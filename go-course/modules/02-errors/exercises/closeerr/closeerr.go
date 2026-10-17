// Package closeerr shows how to write data without losing errors from
// Flush and Close, using defer and a named result.
package closeerr

import (
	"errors"
	"io"
)

// WriteLines writes each line followed by '\n' to w through a bufio.Writer,
// flushes the buffer, and closes w exactly once, on every path.
//
// Error rules:
//   - A failed write or flush is returned wrapped ("write: ..." or
//     "flush: ..."). Note that with buffering, a failure of the underlying
//     writer often only surfaces at Flush.
//   - If Close fails too, its error is joined with the first one, so both
//     stay visible to errors.Is.
//   - If only Close fails, the result is "close: <close error>".
//   - Otherwise the result is nil.
func WriteLines(w io.WriteCloser, lines []string) (err error) {
	// TODO: Implement WriteLines using a named return and defer.
	// Use bufio.NewWriter to buffer writes.
	// Defer w.Close() in a closure that also checks and handles the close error.
	// Join write/flush errors with close errors.
	return errors.New("not implemented")
}

// SaveFile creates (or truncates) the file at path and writes lines to it
// with WriteLines. Errors from os.Create are returned unchanged: the
// *fs.PathError already names the operation and the path.
func SaveFile(path string, lines []string) error {
	// TODO: Implement SaveFile.
	// Open the file with os.Create.
	// Call WriteLines and return its result.
	return errors.New("not implemented")
}
