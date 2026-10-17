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

// Read implements io.Reader. It reads from the underlying reader into p and
// transforms, in place, exactly the n bytes that were read. Any error from the
// underlying reader (including io.EOF) is returned unchanged, together with
// the n bytes, as the io.Reader contract requires.
func (r *Reader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	for i, b := range p[:n] {
		p[i] = rot13(b)
	}
	return n, err
}

// rot13 rotates an ASCII letter 13 places, preserving case.
func rot13(b byte) byte {
	switch {
	case 'a' <= b && b <= 'z':
		return 'a' + (b-'a'+13)%26
	case 'A' <= b && b <= 'Z':
		return 'A' + (b-'A'+13)%26
	default:
		return b
	}
}
