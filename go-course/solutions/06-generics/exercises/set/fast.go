package set

// IntersectFast returns the same result as Intersect but always iterates the
// smaller of the two sets and probes the larger, so it costs
// O(min(len(s), len(o))) lookups instead of O(len(s)). It also sizes the
// result map up front, avoiding incremental map growth. The asymmetry makes
// the code a little harder to read, which is why the readable version
// doesn't bother.
func (s *Set[T]) IntersectFast(o *Set[T]) *Set[T] {
	small, large := s, o
	if small.Len() > large.Len() {
		small, large = large, small
	}
	out := &Set[T]{m: make(map[T]struct{}, small.Len())}
	if small == nil || large == nil {
		return out
	}
	for v := range small.m { // direct map range: no iterator closure
		if _, ok := large.m[v]; ok {
			out.m[v] = struct{}{}
		}
	}
	return out
}
