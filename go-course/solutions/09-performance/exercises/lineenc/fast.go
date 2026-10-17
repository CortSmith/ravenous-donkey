package lineenc

import (
	"fmt"
	"io"
	"strconv"
	"sync"
)

// AppendRecord appends the encoded line for r to dst. It is the building block
// for both Encoder.Encode and WriteRecord, and lets callers batch many records
// into one buffer before a single Write.
func AppendRecord(dst []byte, r Record) []byte {
	dst = append(dst, "name="...)
	dst = strconv.AppendQuote(dst, r.Name)
	dst = append(dst, " score="...)
	dst = strconv.AppendInt(dst, int64(r.Score), 10)
	dst = append(dst, " passed="...)
	dst = strconv.AppendBool(dst, r.Passed)
	return append(dst, '\n')
}

// bufPool holds *[]byte, not []byte: putting a slice header directly into an
// interface would itself allocate (staticcheck SA6002).
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 128)
		return &b
	},
}

// maxPooledCap stops one huge record from pinning a huge buffer in the pool
// forever (see golang/go#23199).
const maxPooledCap = 4 << 10

// WriteRecord is a stateless, goroutine-safe alternative to Encoder: it
// borrows a buffer from a sync.Pool. Use it when there is no natural owner for
// a buffer (e.g. many goroutines writing records to different writers).
// In single-goroutine code the Encoder is simpler and at least as fast.
func WriteRecord(w io.Writer, r Record) error {
	bp := bufPool.Get().(*[]byte)
	b := AppendRecord((*bp)[:0], r)
	_, err := w.Write(b)
	if cap(b) <= maxPooledCap {
		*bp = b
		bufPool.Put(bp)
	}
	if err != nil {
		return fmt.Errorf("lineenc: write record: %w", err)
	}
	return nil
}
