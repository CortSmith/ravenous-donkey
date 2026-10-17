package rot13

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReader(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"lowercase", "hello", "uryyb"},
		{"uppercase", "HELLO", "URYYB"},
		{"mixed case", "Hello, World!", "Uryyb, Jbeyq!"},
		{"alphabet", "abcdefghijklmnopqrstuvwxyz", "nopqrstuvwxyzabcdefghijklm"},
		{"ALPHABET", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "NOPQRSTUVWXYZABCDEFGHIJKLM"},
		{"boundaries", "@AZ[`az{", "@NM[`nm{"},
		{"digits and punctuation untouched", "0123456789 !?.,;:-_\n\t", "0123456789 !?.,;:-_\n\t"},
		{"utf-8 untouched", "héllo wörld 日本", "uéyyb jöeyq 日本"},
		{"why did the chicken", "Why did the chicken cross the road?", "Jul qvq gur puvpxra pebff gur ebnq?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := io.ReadAll(NewReader(strings.NewReader(tt.in)))
			if err != nil {
				t.Fatalf("ReadAll(NewReader(%q)) error = %v, want nil", tt.in, err)
			}
			if string(got) != tt.want {
				t.Errorf("ReadAll(NewReader(%q)) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestRoundTrip checks that ROT13 applied twice is the identity for every
// possible byte value.
func TestRoundTrip(t *testing.T) {
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	got, err := io.ReadAll(NewReader(NewReader(bytes.NewReader(all))))
	if err != nil {
		t.Fatalf("ReadAll error = %v", err)
	}
	if !bytes.Equal(got, all) {
		t.Errorf("double ROT13 is not the identity:\n got %v\nwant %v", got, all)
	}
}

// TestOnlyTransformsReadBytes guards against the classic bug of transforming
// the whole of p instead of p[:n]: bytes beyond n belong to the caller.
func TestOnlyTransformsReadBytes(t *testing.T) {
	buf := []byte("XXXXXXXXXX")
	r := NewReader(strings.NewReader("abc"))
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Read error = %v", err)
	}
	if n != 3 {
		t.Fatalf("Read returned n = %d, want 3", n)
	}
	if got, want := string(buf), "nopXXXXXXX"; got != want {
		t.Errorf("buffer after Read = %q, want %q (bytes past n must not be modified)", got, want)
	}
}

// TestSmallReads uses readers that misbehave in legal ways: returning one
// byte at a time, or returning data together with io.EOF.
func TestSmallReads(t *testing.T) {
	const in, want = "The Quick Brown Fox", "Gur Dhvpx Oebja Sbk"
	wrappers := map[string]func(io.Reader) io.Reader{
		"OneByteReader": iotest.OneByteReader,
		"DataErrReader": iotest.DataErrReader,
		"HalfReader":    iotest.HalfReader,
	}
	for name, wrap := range wrappers {
		t.Run(name, func(t *testing.T) {
			got, err := io.ReadAll(NewReader(wrap(strings.NewReader(in))))
			if err != nil {
				t.Fatalf("ReadAll error = %v", err)
			}
			if string(got) != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// TestPropagatesErrors checks that errors from the underlying reader are
// returned unchanged (not swallowed, not replaced by io.EOF).
func TestPropagatesErrors(t *testing.T) {
	boom := errors.New("boom")
	r := NewReader(io.MultiReader(strings.NewReader("abc"), iotest.ErrReader(boom)))
	got, err := io.ReadAll(r)
	if !errors.Is(err, boom) {
		t.Fatalf("ReadAll error = %v, want %v", err, boom)
	}
	if string(got) != "nop" {
		t.Errorf("data before error = %q, want %q", got, "nop")
	}
}

// TestIOTest runs the standard library's own conformance checks for readers.
func TestIOTest(t *testing.T) {
	in := strings.Repeat("Go is expressive, concise, clean, and efficient. ", 50)
	want := make([]byte, len(in))
	for i := range len(in) {
		want[i] = refROT13(in[i])
	}
	if err := iotest.TestReader(NewReader(strings.NewReader(in)), want); err != nil {
		t.Error(err)
	}
}

func TestComposesWithIOCopy(t *testing.T) {
	var out strings.Builder
	n, err := io.Copy(&out, NewReader(strings.NewReader("Gophers")))
	if err != nil {
		t.Fatalf("io.Copy error = %v", err)
	}
	if n != 7 || out.String() != "Tbcuref" {
		t.Errorf("io.Copy = (%d, %q), want (7, %q)", n, out.String(), "Tbcuref")
	}
}

// refROT13 is an independent reference implementation used by the tests.
func refROT13(b byte) byte {
	const lower, upper = "abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if i := strings.IndexByte(lower, b); i >= 0 {
		return lower[(i+13)%26]
	}
	if i := strings.IndexByte(upper, b); i >= 0 {
		return upper[(i+13)%26]
	}
	return b
}
