// Package funcs implements the classic higher-order helpers Map, Filter,
// Reduce and GroupBy as generic functions over slices.
package funcs

// Map returns a new slice holding f applied to every element of s, in order.
// The result has the same length as s. Map never modifies s.
func Map[S ~[]E, E, R any](s S, f func(E) R) []R {
	out := make([]R, 0, len(s))
	for _, v := range s {
		out = append(out, f(v))
	}
	return out
}

// Filter returns a new slice holding the elements of s for which keep
// returns true, in their original order. The result has the same named
// slice type as s. Filter never modifies s or its backing array.
func Filter[S ~[]E, E any](s S, keep func(E) bool) S {
	var out S
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

// Reduce folds s from left to right: it starts with acc = init and, for every
// element v, sets acc = f(acc, v). It returns the final accumulator, which is
// init itself when s is empty.
func Reduce[S ~[]E, E, A any](s S, init A, f func(A, E) A) A {
	acc := init
	for _, v := range s {
		acc = f(acc, v)
	}
	return acc
}

// GroupBy partitions s by the key returned from key. Within each group the
// elements keep their original relative order. GroupBy always returns a
// non-nil map (empty when s is empty).
func GroupBy[S ~[]E, E any, K comparable](s S, key func(E) K) map[K]S {
	groups := make(map[K]S)
	for _, v := range s {
		k := key(v)
		groups[k] = append(groups[k], v)
	}
	return groups
}
