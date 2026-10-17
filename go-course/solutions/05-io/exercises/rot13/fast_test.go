package rot13

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestFastReaderMatchesReader(t *testing.T) {
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	want, _ := io.ReadAll(NewReader(bytes.NewReader(all)))
	got, err := io.ReadAll(NewFastReader(bytes.NewReader(all)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("FastReader output differs from Reader:\n got %v\nwant %v", got, want)
	}
}

var benchInput = []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. 0123456789\n", 20000))

func benchmarkReader(b *testing.B, mk func(io.Reader) io.Reader) {
	b.SetBytes(int64(len(benchInput)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := io.Copy(io.Discard, mk(bytes.NewReader(benchInput))); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReader(b *testing.B) {
	benchmarkReader(b, func(r io.Reader) io.Reader { return NewReader(r) })
}

func BenchmarkFastReader(b *testing.B) {
	benchmarkReader(b, func(r io.Reader) io.Reader { return NewFastReader(r) })
}
