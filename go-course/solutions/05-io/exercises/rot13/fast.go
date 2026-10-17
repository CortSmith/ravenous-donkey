package rot13

import "io"

// table maps every byte to its ROT13 image. It is computed once at package
// initialisation by a pure function (no side effects), which is fine.
var table = func() (t [256]byte) {
	for i := range t {
		t[i] = rot13(byte(i))
	}
	return t
}()

// FastReader is like Reader but uses a 256-entry lookup table instead of
// branching on every byte.
type FastReader struct {
	r io.Reader
}

// NewFastReader returns a FastReader that ROT13-transforms the bytes read from r.
func NewFastReader(r io.Reader) *FastReader {
	return &FastReader{r: r}
}

// Read implements io.Reader.
func (r *FastReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	p = p[:n] // lets the compiler prove i < len(p) below (bounds-check elimination)
	for i, b := range p {
		p[i] = table[b] // byte index into [256]byte: no bounds check needed
	}
	return n, err
}
