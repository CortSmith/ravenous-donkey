package appendints

import (
	"slices"
	"strconv"
)

// maxIntLen is the longest base-10 int64: "-9223372036854775808".
const maxIntLen = 20

// AppendIntsFast behaves exactly like AppendInts but grows dst once, up
// front, to a worst-case size. With a nil or small dst, AppendInts lets append
// double the slice repeatedly (about log2(n) allocations and copies);
// AppendIntsFast does a single allocation. The cost is over-allocation: for
// small numbers most of the reserved space is unused.
func AppendIntsFast(dst []byte, nums []int, sep byte) []byte {
	if len(nums) == 0 {
		return dst
	}
	dst = slices.Grow(dst, len(nums)*(maxIntLen+1))
	for i, n := range nums {
		if i > 0 {
			dst = append(dst, sep)
		}
		dst = strconv.AppendInt(dst, int64(n), 10)
	}
	return dst
}
