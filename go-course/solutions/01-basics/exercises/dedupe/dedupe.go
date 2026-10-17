// Package dedupe removes duplicate elements from slices.
package dedupe

// Dedupe returns the distinct elements of s in order of first appearance.
//
// s is not modified, and the result never shares a backing array with s, so
// callers may freely modify or append to either. A nil or empty s yields an
// empty (length 0) result.
func Dedupe(s []string) []string {
	seen := make(map[string]struct{}, len(s)) // size hint avoids rehashing
	out := make([]string, 0, len(s))          // upper bound: every element unique
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// CompactInPlace removes consecutive duplicate values from s in place and
// returns the shortened slice, which shares s's backing array. Applied to a
// sorted slice it removes all duplicates. Elements of s beyond the returned
// length are unspecified.
func CompactInPlace(s []int) []int {
	if len(s) < 2 {
		return s
	}
	k := 1 // s[:k] is the compacted prefix
	for i := 1; i < len(s); i++ {
		if s[i] != s[k-1] {
			s[k] = s[i]
			k++
		}
	}
	return s[:k]
}
