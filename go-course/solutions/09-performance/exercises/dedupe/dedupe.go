// Package dedupe removes duplicate strings while preserving order.
package dedupe

// Unique returns the distinct elements of items in order of first
// appearance. items is not modified. The result for an empty input has
// length 0 (it may be nil).
//
// Both the set and the result are sized for the worst case (every element
// distinct), so neither ever grows: the allocation count is a small constant
// instead of growing with log(len(items)). The trade-off is memory: with many
// duplicates the result slice has unused capacity.
func Unique(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		if _, ok := seen[it]; ok {
			continue
		}
		seen[it] = struct{}{}
		out = append(out, it)
	}
	return out
}
