package iterx

import "iter"

// FilterMap fuses Filter and Map into one adapter: one closure layer and one
// yield call per kept element instead of two. Use it when profiling shows the
// pipeline itself is hot; otherwise composing Filter and Map reads better.
func FilterMap[T, R any](seq iter.Seq[T], keep func(T) bool, f func(T) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		for v := range seq {
			if keep(v) && !yield(f(v)) {
				return
			}
		}
	}
}
