// Package closeerr shows how to write data without losing errors from
// Flush and Close, using defer and a named result.
package closeerr

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
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
	defer func() {
		if cerr := w.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close: %w", cerr))
		}
	}()

	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.WriteString(line); err != nil {
			return fmt.Errorf("write: %w", err)
		}
		if err := bw.WriteByte('\n'); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	return nil
}

// SaveFile creates (or truncates) the file at path and writes lines to it
// with WriteLines. Errors from os.Create are returned unchanged: the
// *fs.PathError already names the operation and the path.
func SaveFile(path string, lines []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return WriteLines(f, lines)
}
