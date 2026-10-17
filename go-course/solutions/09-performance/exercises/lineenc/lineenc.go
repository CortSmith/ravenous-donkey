// Package lineenc writes records as key=value text lines.
package lineenc

import (
	"fmt"
	"io"
	"strconv"
)

// Record is one result row.
type Record struct {
	Name   string
	Score  int
	Passed bool
}

// Encoder writes Records to an underlying io.Writer, one per line, in the
// form:
//
//	name="alice" score=42 passed=true
//
// Name is always quoted using Go syntax (exactly as strconv.Quote does).
// An Encoder is not safe for concurrent use.
type Encoder struct {
	w   io.Writer
	buf []byte // reused by every Encode call; never handed out
}

// NewEncoder returns an Encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode writes r as a single line terminated by '\n' using exactly one call
// to the underlying writer's Write method. If Write fails, Encode returns an
// error that wraps the writer's error.
//
// Encode formats into e.buf, truncated to length zero but keeping its
// capacity, so once the buffer has grown to fit the longest line it does not
// allocate again. That is safe because the io.Writer contract forbids Write
// from retaining the slice it is given.
func (e *Encoder) Encode(r Record) error {
	b := e.buf[:0]
	b = append(b, "name="...)
	b = strconv.AppendQuote(b, r.Name)
	b = append(b, " score="...)
	b = strconv.AppendInt(b, int64(r.Score), 10)
	b = append(b, " passed="...)
	b = strconv.AppendBool(b, r.Passed)
	b = append(b, '\n')
	e.buf = b // keep any growth for next time

	if _, err := e.w.Write(b); err != nil {
		return fmt.Errorf("lineenc: write record: %w", err)
	}
	return nil
}
