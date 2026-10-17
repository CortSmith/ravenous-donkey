// Package set provides a generic hash set.
//
// Run the tests with:
//
//	go test ./modules/06-generics/exercises/set/
package set

import (
	"cmp"
	"iter"
)

// Set is an unordered collection of distinct values. The zero value is an
// empty set ready to use. A nil *Set is treated as an empty set by every
// read-only method (Contains, Len, All, Union, Intersect, Difference).
//
// A Set must not be copied after first use and is not safe for concurrent
// mutation.
type Set[T comparable] struct {
	// map[T]struct{} is the idiomatic set representation: struct{} takes no
	// space, and T must be comparable to be a map key.
	m map[T]struct{}
}

// Of returns a new set holding the given items. Duplicates are collapsed.
func Of[T comparable](items ...T) *Set[T] {
	// TODO
	return new(Set[T])
}

// Add inserts v and reports whether it was newly added.
func (s *Set[T]) Add(v T) bool {
	// TODO: the zero value must work, so s.m may still be nil here.
	return false
}

// Remove deletes v and reports whether it was present.
func (s *Set[T]) Remove(v T) bool {
	// TODO
	return false
}

// Contains reports whether v is in the set.
func (s *Set[T]) Contains(v T) bool {
	// TODO: s itself may be nil.
	return false
}

// Len returns the number of elements in the set.
func (s *Set[T]) Len() int {
	// TODO
	return 0
}

// All returns an iterator over the elements in unspecified order.
// Mutating the set during iteration has the same semantics as mutating a
// map during a range loop.
func (s *Set[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		// TODO: yield every element; stop as soon as yield returns false.
	}
}

// Union returns a new set holding every element of s or o.
// Neither input is modified.
func (s *Set[T]) Union(o *Set[T]) *Set[T] {
	// TODO
	return new(Set[T])
}

// Intersect returns a new set holding the elements present in both s and o.
// Neither input is modified.
func (s *Set[T]) Intersect(o *Set[T]) *Set[T] {
	// TODO
	return new(Set[T])
}

// Difference returns a new set holding the elements of s that are not in o.
// Neither input is modified.
func (s *Set[T]) Difference(o *Set[T]) *Set[T] {
	// TODO
	return new(Set[T])
}

// Sorted returns the elements of s as a sorted slice.
//
// Question for you: why is this a function and not a method on Set?
func Sorted[T cmp.Ordered](s *Set[T]) []T {
	// TODO: the slices package can turn an iter.Seq into a sorted slice in
	// one call.
	return nil
}
