// Package iterx provides lazy adapters over iter.Seq: the building blocks of
// iterator pipelines.
//
// Every function here is lazy: it does no work until the returned sequence
// is ranged over, and it pulls from its source only as many elements as the
// consumer asks for.
//
// Run the tests with:
//
//	go test ./modules/06-generics/exercises/iterx/
//
// The one rule you must not break: when yield returns false, return
// immediately and never call yield again.
package iterx

import "iter"

// Naturals returns the infinite sequence 0, 1, 2, ... It is only useful
// with a consumer that stops, such as Take or a loop with break.
func Naturals() iter.Seq[int] {
	return func(yield func(int) bool) {
		// TODO
	}
}

// Filter returns a sequence of the elements of seq for which keep returns
// true.
func Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		// TODO: you can range over seq here: `for v := range seq { ... }`.
	}
}

// Map returns a sequence of f applied to each element of seq.
func Map[T, R any](seq iter.Seq[T], f func(T) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		// TODO
	}
}

// Take returns a sequence of at most the first n elements of seq. It never
// pulls more than n elements from seq, and pulls none when n <= 0.
func Take[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		// TODO: where you declare your counter matters. The returned
		// sequence may be ranged over more than once.
	}
}

// Enumerate pairs each element of seq with its zero-based position.
func Enumerate[T any](seq iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		// TODO
	}
}
