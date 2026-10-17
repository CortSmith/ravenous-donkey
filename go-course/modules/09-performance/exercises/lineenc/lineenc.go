// Package lineenc writes records as key=value text lines.
package lineenc

import (
	"fmt"
	"io"
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
	w io.Writer
	// TODO: add a reusable buffer here.
}

// NewEncoder returns an Encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode writes r as a single line terminated by '\n' using exactly one call
// to the underlying writer's Write method. If Write fails, Encode returns an
// error that wraps the writer's error.
//
// This version is CORRECT but SLOW: fmt.Sprintf boxes its arguments and
// allocates a new string, and []byte(line) allocates and copies it again —
// for every single record.
//
// TODO: keep the behaviour, but make Encode allocation-free in steady state by
// formatting into a []byte owned by the Encoder and reusing it between calls.
func (e *Encoder) Encode(r Record) error {
	line := fmt.Sprintf("name=%q score=%d passed=%t\n", r.Name, r.Score, r.Passed)
	if _, err := e.w.Write([]byte(line)); err != nil {
		return fmt.Errorf("lineenc: write record: %w", err)
	}
	return nil
}
