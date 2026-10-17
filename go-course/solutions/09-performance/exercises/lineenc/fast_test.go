package lineenc

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"testing"
)

func TestWriteRecordMatchesEncoder(t *testing.T) {
	r := Record{`x "y"`, -5, true}
	var a, b bytes.Buffer
	if err := NewEncoder(&a).Encode(r); err != nil {
		t.Fatal(err)
	}
	if err := WriteRecord(&b, r); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Errorf("WriteRecord wrote %q, Encoder wrote %q", b.String(), a.String())
	}
}

func TestWriteRecordConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 200 {
				var buf bytes.Buffer
				r := Record{fmt.Sprint("g", i), j, j%2 == 0}
				if err := WriteRecord(&buf, r); err != nil {
					t.Error(err)
					return
				}
				if want := string(AppendRecord(nil, r)); buf.String() != want {
					t.Errorf("WriteRecord = %q, want %q", buf.String(), want)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// appendfEncoder is the tempting half-fix: reuse a buffer, but keep fmt.
type appendfEncoder struct {
	w   io.Writer
	buf []byte
}

func (e *appendfEncoder) Encode(r Record) error {
	e.buf = fmt.Appendf(e.buf[:0], "name=%q score=%d passed=%t\n", r.Name, r.Score, r.Passed)
	_, err := e.w.Write(e.buf)
	return err
}

func BenchmarkEncodeVariants(b *testing.B) {
	r := Record{"bob the builder", 1234567, true}
	b.Run("Sprintf", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			line := fmt.Sprintf("name=%q score=%d passed=%t\n", r.Name, r.Score, r.Passed)
			io.Discard.Write([]byte(line))
		}
	})
	b.Run("AppendfReusedBuf", func(b *testing.B) {
		e := &appendfEncoder{w: io.Discard}
		b.ReportAllocs()
		for b.Loop() {
			e.Encode(r)
		}
	})
	b.Run("Encoder", func(b *testing.B) {
		e := NewEncoder(io.Discard)
		b.ReportAllocs()
		for b.Loop() {
			e.Encode(r)
		}
	})
	b.Run("WriteRecordPool", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			WriteRecord(io.Discard, r)
		}
	})
}
