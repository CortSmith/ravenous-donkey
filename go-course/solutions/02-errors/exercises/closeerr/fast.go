package closeerr

import (
	"errors"
	"fmt"
	"io"
)

// WriteLinesFast has the same contract as WriteLines but builds the whole
// output in one exactly-sized []byte and issues a single Write.
//
// bufio.NewWriter always allocates a 4096-byte buffer, which dominates the
// cost for small inputs; and for inputs larger than 4 KiB it issues several
// Writes (several syscalls on a real file). Here we pay one allocation of
// exactly the needed size and one Write. The trade-off: memory use grows
// with the input, so this is only appropriate when the data is already in
// memory anyway (as it is here: lines []string).
func WriteLinesFast(w io.WriteCloser, lines []string) (err error) {
	defer func() {
		if cerr := w.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close: %w", cerr))
		}
	}()

	n := len(lines) // one '\n' per line
	for _, l := range lines {
		n += len(l)
	}
	if n == 0 {
		return nil
	}
	buf := make([]byte, 0, n)
	for _, l := range lines {
		buf = append(buf, l...)
		buf = append(buf, '\n')
	}
	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
