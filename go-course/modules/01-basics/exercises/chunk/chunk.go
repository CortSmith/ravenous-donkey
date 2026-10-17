// Package chunk splits and rotates slices while staying aware of aliasing.
package chunk

// Chunk splits s into consecutive sub-slices of length size; the final chunk
// is shorter if len(s) is not a multiple of size. Chunk returns nil if size is
// less than 1 or s is empty.
//
// The chunks are views into s, not copies: writing chunk[i][j] writes the
// corresponding element of s. However, every chunk's capacity equals its
// length, so appending to one chunk allocates a fresh array instead of
// overwriting the first element of the next chunk.
func Chunk(s []int, size int) [][]int {
	// TODO: slice s without copying, but cap each chunk with a full slice
	// expression s[lo:hi:max].
	return nil
}

// RotateLeft rotates the elements of s left by k positions, in place: the
// element at index k moves to index 0 and the first k elements move to the
// end. A negative k rotates right. k may be larger than len(s); rotating by
// len(s) is a no-op.
func RotateLeft(s []int, k int) {
	// TODO: normalize k into [0, len(s)) first. Careful: in Go, -1 % 5 == -1.
}
