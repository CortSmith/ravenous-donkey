// Package iterx provides lazy adapters over iter.Seq: the building blocks of
// iterator pipelines.
//
// Every function here is lazy: it does no work until the returned sequence
// is ranged over, and it pulls from its source only as many elements as the
// consumer asks for.
package iterx

import "iter"

// Naturals returns the infinite sequence 0, 1, 2, ... It is only useful
// with a consumer that stops, such as Take or a loop with break.
func Naturals() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; ; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

// Filter returns a sequence of the elements of seq for which keep returns
// true.
func Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if keep(v) && !yield(v) {
				return
			}
		}
	}
}

// Map returns a sequence of f applied to each element of seq.
func Map[T, R any](seq iter.Seq[T], f func(T) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		for v := range seq {
			if !yield(f(v)) {
				return
			}
		}
	}
}

// Take returns a sequence of at most the first n elements of seq. It never
// pulls more than n elements from seq, and pulls none when n <= 0.
func Take[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		if n <= 0 {
			return
		}
		taken := 0 // declared inside the closure so each range starts fresh
		for v := range seq {
			if !yield(v) {
				return
			}
			taken++
			if taken == n {
				return
			}
		}
	}
}

// Enumerate pairs each element of seq with its zero-based position.
func Enumerate[T any](seq iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		i := 0
		for v := range seq {
			if !yield(i, v) {
				return
			}
			i++
		}
	}
}
