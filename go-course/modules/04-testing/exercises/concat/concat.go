// Package concat contains several ways to join strings so that you can
// benchmark them against each other. Every function must return exactly
// what strings.Join(elems, sep) returns.
//
// This package has 2 bugs: one makes a function return the wrong answer,
// the other makes it allocate far more than its doc comment promises.
// Benchmarking wrong code is pointless, so write the correctness test
// first. See concat_test.go.
package concat

import (
	"bytes"
	"strings"
)

// JoinPlus joins elems with sep using the + operator. Each += copies the
// whole string built so far, so it is O(n²) in the output length.
func JoinPlus(elems []string, sep string) string {
	var s string
	for i, e := range elems {
		if i > 0 {
			s += sep
		}
		s += e
	}
	return s
}

// JoinBuffer joins elems with sep using a bytes.Buffer that grows as
// needed.
func JoinBuffer(elems []string, sep string) string {
	var buf bytes.Buffer
	for i, e := range elems {
		if i > 0 {
			buf.WriteString(sep)
		}
		buf.WriteString(e)
	}
	return buf.String()
}

// JoinBuilder joins elems with sep using a strings.Builder that is sized
// up front, so it allocates exactly once for a non-empty result.
func JoinBuilder(elems []string, sep string) string {
	var b strings.Builder
	b.Grow(len(elems) * len(sep))
	for i, e := range elems {
		if e == "" {
			continue // nothing to write
		}
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(e)
	}
	return b.String()
}
