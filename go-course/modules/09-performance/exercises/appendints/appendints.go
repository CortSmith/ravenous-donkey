// Package appendints formats integers into a caller-supplied byte slice.
package appendints

import "fmt"

// AppendInts appends the base-10 representation of each element of nums to
// dst, separated by sep, and returns the extended slice. Existing contents of
// dst are preserved. If nums is empty, dst is returned unchanged.
//
// This version is CORRECT but SLOW: fmt.Sprintf boxes n into an interface
// (an allocation for most values), parses the format string at run time, and
// returns a brand-new string that is immediately copied into dst and thrown
// away.
//
// TODO: keep the behaviour, but make AppendInts allocation-free whenever dst
// already has enough capacity. Look at the strconv package's Append* family.
func AppendInts(dst []byte, nums []int, sep byte) []byte {
	for i, n := range nums {
		if i > 0 {
			dst = append(dst, sep)
		}
		dst = append(dst, fmt.Sprintf("%d", n)...)
	}
	return dst
}
