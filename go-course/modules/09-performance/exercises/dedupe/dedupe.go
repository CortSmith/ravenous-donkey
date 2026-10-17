// Package dedupe removes duplicate strings while preserving order.
package dedupe

// Unique returns the distinct elements of items in order of first
// appearance. items is not modified. The result for an empty input has
// length 0 (it may be nil).
//
// This version is CORRECT but SLOW: both the map and the result slice start
// empty and are grown again and again as elements are added. Each growth step
// allocates a bigger table/array and copies (or rehashes) everything so far.
//
// TODO: keep the behaviour, but cut the allocations by sizing the map and the
// slice up front. You know an upper bound on how big each can get.
func Unique(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, it := range items {
		if !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	return out
}
