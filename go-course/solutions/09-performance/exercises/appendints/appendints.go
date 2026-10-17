// Package appendints formats integers into a caller-supplied byte slice.
package appendints

import "strconv"

// AppendInts appends the base-10 representation of each element of nums to
// dst, separated by sep, and returns the extended slice. Existing contents of
// dst are preserved. If nums is empty, dst is returned unchanged.
//
// When dst has enough spare capacity, AppendInts does not allocate:
// strconv.AppendInt writes digits straight into dst, with no intermediate
// string and no interface boxing.
func AppendInts(dst []byte, nums []int, sep byte) []byte {
	for i, n := range nums {
		if i > 0 {
			dst = append(dst, sep)
		}
		dst = strconv.AppendInt(dst, int64(n), 10)
	}
	return dst
}
