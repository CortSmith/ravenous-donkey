package dedupe

import "slices"

// smallN is the size below which a linear scan beats hashing. It was picked
// from BenchmarkUniqueSmall on the author's machine; re-measure on yours.
const smallN = 32

// UniqueFast returns the same result as Unique. For small inputs it skips the
// map entirely and checks membership with a linear scan of the output, which
// is cache-friendly and costs a single allocation (the result slice).
// It is O(n²) in the worst case, so the cutoff matters.
func UniqueFast(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	if len(items) > smallN {
		return Unique(items)
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if !slices.Contains(out, it) {
			out = append(out, it)
		}
	}
	return out
}

// Deduper reuses its set between calls. After the first few calls it does
// not allocate at all (when dst has capacity), because clear keeps the map's
// storage. A Deduper must not be used by multiple goroutines at once.
type Deduper struct {
	seen map[string]struct{}
}

// AppendUnique appends the distinct elements of items, in first-appearance
// order, to dst and returns the extended slice.
func (d *Deduper) AppendUnique(dst, items []string) []string {
	if d.seen == nil {
		d.seen = make(map[string]struct{}, len(items))
	} else {
		clear(d.seen)
	}
	for _, it := range items {
		if _, ok := d.seen[it]; ok {
			continue
		}
		d.seen[it] = struct{}{}
		dst = append(dst, it)
	}
	return dst
}
