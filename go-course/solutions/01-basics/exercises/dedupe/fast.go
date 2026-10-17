package dedupe

// smallDedupeLimit is the input size up to which a linear scan of the output
// beats hashing. Measured with BenchmarkDedupe*/small and /large; the crossover
// on amd64 sits somewhere between 16 and 64 short strings.
const smallDedupeLimit = 32

// DedupeFast behaves like Dedupe but avoids building a map for small inputs.
//
// For short slices, comparing each element against the (short) output is
// O(n²) in theory but much cheaper in practice: no hashing, no map buckets to
// allocate, and everything stays in cache. Larger inputs fall back to Dedupe.
func DedupeFast(s []string) []string {
	if len(s) > smallDedupeLimit {
		return Dedupe(s)
	}
	out := make([]string, 0, len(s))
outer:
	for _, v := range s {
		for _, u := range out {
			if u == v {
				continue outer
			}
		}
		out = append(out, v)
	}
	return out
}
