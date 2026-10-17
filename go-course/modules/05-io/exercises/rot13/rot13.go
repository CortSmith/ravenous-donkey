// Package rot13 provides an io.Reader that applies the ROT13 substitution
// cipher to the bytes flowing through it.
package rot13

import "io"

// Reader wraps another io.Reader and ROT13-encodes every ASCII letter that is
// read through it. All other bytes (digits, punctuation, UTF-8 continuation
// bytes, ...) pass through unchanged.
//
// Because ROT13 is its own inverse, a Reader also decodes.
type Reader struct {
	r io.Reader
}

// NewReader returns a Reader that ROT13-transforms the bytes read from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

// Read implements io.Reader.
//
// TODO: read from the underlying reader into p, transform the bytes that were
// actually read (p[:n], not all of p!), and return n together with whatever
// error the underlying reader returned. Remember: a Read may return n > 0 AND
// a non-nil error (even io.EOF) in the same call.
func (r *Reader) Read(p []byte) (int, error) {
	return 0, io.EOF
}
