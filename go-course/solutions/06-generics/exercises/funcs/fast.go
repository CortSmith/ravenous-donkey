package funcs

// MapFast is Map without append: it sizes the result once and assigns by
// index, which lets the compiler drop append's capacity check. The gain is
// small; see REVIEW.md for measurements.
func MapFast[S ~[]E, E, R any](s S, f func(E) R) []R {
	out := make([]R, len(s))
	for i, v := range s {
		out[i] = f(v)
	}
	return out
}

// FilterFast allocates the worst-case capacity (len(s)) up front, so it does
// exactly one allocation however many elements survive. The cost is memory:
// if keep rejects most elements, the result pins a mostly-empty backing
// array. Callers that keep results long-term can slices.Clip them.
func FilterFast[S ~[]E, E any](s S, keep func(E) bool) S {
	out := make(S, 0, len(s))
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

// FilterInPlace reuses s's backing array and therefore DESTROYS s. It does
// zero allocations. It is what slices.DeleteFunc does (with the predicate
// inverted), and is only correct when the caller owns s and will not use it
// again.
func FilterInPlace[S ~[]E, E any](s S, keep func(E) bool) S {
	out := s[:0]
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	clear(s[len(out):]) // let the GC reclaim anything the dropped tail pointed at
	return out
}
