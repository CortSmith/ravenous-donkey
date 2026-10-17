// Package funcs implements the classic higher-order helpers Map, Filter,
// Reduce and GroupBy as generic functions over slices.
//
// Read the "Type parameters" and "The S ~[]E pattern" sections of the module
// README before starting. Run the tests with:
//
//	go test ./modules/06-generics/exercises/funcs/
package funcs

// Map returns a new slice holding f applied to every element of s, in order.
// The result has the same length as s. Map never modifies s.
//
// The type parameter list reads: S is some slice type whose elements are E
// (it may be a named type like `type Names []string`), and R is the element
// type of the result.
func Map[S ~[]E, E, R any](s S, f func(E) R) []R {
	// TODO: allocate the result with the right capacity, then fill it.
	return nil
}

// Filter returns a new slice holding the elements of s for which keep
// returns true, in their original order. The result has the same named
// slice type as s. Filter never modifies s or its backing array.
func Filter[S ~[]E, E any](s S, keep func(E) bool) S {
	// TODO: careful: the "filter in place with s[:0]" trick overwrites s,
	// which this function promises not to do.
	return nil
}

// Reduce folds s from left to right: it starts with acc = init and, for every
// element v, sets acc = f(acc, v). It returns the final accumulator, which is
// init itself when s is empty.
func Reduce[S ~[]E, E, A any](s S, init A, f func(A, E) A) A {
	// TODO
	return init
}

// GroupBy partitions s by the key returned from key. Within each group the
// elements keep their original relative order. GroupBy always returns a
// non-nil map (empty when s is empty).
func GroupBy[S ~[]E, E any, K comparable](s S, key func(E) K) map[K]S {
	// TODO: why must K be comparable here, while E can be any?
	return nil
}
