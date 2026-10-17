// Package dedupe removes duplicate elements from slices.
package dedupe

// Dedupe returns the distinct elements of s in order of first appearance.
//
// s is not modified, and the result never shares a backing array with s, so
// callers may freely modify or append to either. A nil or empty s yields an
// empty (length 0) result.
func Dedupe(s []string) []string {
	// TODO: track what you have already seen. Which map value type is the
	// idiomatic choice for a set? How big can the result possibly be?
	return nil
}

// CompactInPlace removes consecutive duplicate values from s in place and
// returns the shortened slice, which shares s's backing array. Applied to a
// sorted slice it removes all duplicates. Elements of s beyond the returned
// length are unspecified.
func CompactInPlace(s []int) []int {
	// TODO: keep a write index k; s[:k] is the compacted prefix. Do not
	// allocate.
	return nil
}
