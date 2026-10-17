// Package chunk splits and rotates slices while staying aware of aliasing.
package chunk

import "slices"

// Chunk splits s into consecutive sub-slices of length size; the final chunk
// is shorter if len(s) is not a multiple of size. Chunk returns nil if size is
// less than 1 or s is empty.
//
// The chunks are views into s, not copies: writing chunk[i][j] writes the
// corresponding element of s. However, every chunk's capacity equals its
// length, so appending to one chunk allocates a fresh array instead of
// overwriting the first element of the next chunk.
func Chunk(s []int, size int) [][]int {
	if size < 1 || len(s) == 0 {
		return nil
	}
	chunks := make([][]int, 0, (len(s)+size-1)/size) // ceil(len/size)
	for len(s) > 0 {
		n := min(size, len(s))
		chunks = append(chunks, s[:n:n]) // full slice expression caps capacity
		s = s[n:]
	}
	return chunks
}

// RotateLeft rotates the elements of s left by k positions, in place: the
// element at index k moves to index 0 and the first k elements move to the
// end. A negative k rotates right. k may be larger than len(s); rotating by
// len(s) is a no-op.
func RotateLeft(s []int, k int) {
	n := len(s)
	if n == 0 {
		return
	}
	k %= n // Go's % keeps the sign of k, so -1 % 5 == -1
	if k < 0 {
		k += n
	}
	if k == 0 {
		return
	}
	head := slices.Clone(s[:k]) // must copy: the next line overwrites s[:k]
	copy(s, s[k:])
	copy(s[n-k:], head)
}
