package chunk

import "slices"

// RotateLeftFast behaves exactly like RotateLeft but never allocates.
//
// It uses the classic three-reversal trick: to rotate AB into BA, reverse A,
// reverse B, then reverse the whole slice: (AᴿBᴿ)ᴿ = BA. Every element is
// swapped about twice, all in place, so the cost is O(n) time and O(1) extra
// space, versus RotateLeft's O(k) temporary copy.
func RotateLeftFast(s []int, k int) {
	n := len(s)
	if n == 0 {
		return
	}
	k %= n
	if k < 0 {
		k += n
	}
	slices.Reverse(s[:k])
	slices.Reverse(s[k:])
	slices.Reverse(s)
}
