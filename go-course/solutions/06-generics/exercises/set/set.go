// Package set provides a generic hash set.
package set

import (
	"cmp"
	"iter"
	"slices"
)

// Set is an unordered collection of distinct values. The zero value is an
// empty set ready to use. A nil *Set is treated as an empty set by every
// read-only method (Contains, Len, All, Union, Intersect, Difference).
//
// A Set must not be copied after first use and is not safe for concurrent
// mutation.
type Set[T comparable] struct {
	m map[T]struct{}
}

// Of returns a new set holding the given items. Duplicates are collapsed.
func Of[T comparable](items ...T) *Set[T] {
	s := &Set[T]{m: make(map[T]struct{}, len(items))}
	for _, v := range items {
		s.m[v] = struct{}{}
	}
	return s
}

// Add inserts v and reports whether it was newly added.
func (s *Set[T]) Add(v T) bool {
	if s.m == nil {
		s.m = make(map[T]struct{})
	}
	if _, ok := s.m[v]; ok {
		return false
	}
	s.m[v] = struct{}{}
	return true
}

// Remove deletes v and reports whether it was present.
func (s *Set[T]) Remove(v T) bool {
	if _, ok := s.m[v]; !ok { // reading a nil map is fine
		return false
	}
	delete(s.m, v)
	return true
}

// Contains reports whether v is in the set.
func (s *Set[T]) Contains(v T) bool {
	if s == nil {
		return false
	}
	_, ok := s.m[v]
	return ok
}

// Len returns the number of elements in the set.
func (s *Set[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.m)
}

// All returns an iterator over the elements in unspecified order.
// Mutating the set during iteration has the same semantics as mutating a
// map during a range loop.
func (s *Set[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if s == nil {
			return
		}
		for v := range s.m {
			if !yield(v) {
				return
			}
		}
	}
}

// Union returns a new set holding every element of s or o.
// Neither input is modified.
func (s *Set[T]) Union(o *Set[T]) *Set[T] {
	out := &Set[T]{m: make(map[T]struct{}, s.Len()+o.Len())}
	for v := range s.All() {
		out.m[v] = struct{}{}
	}
	for v := range o.All() {
		out.m[v] = struct{}{}
	}
	return out
}

// Intersect returns a new set holding the elements present in both s and o.
// Neither input is modified.
func (s *Set[T]) Intersect(o *Set[T]) *Set[T] {
	out := &Set[T]{}
	for v := range s.All() {
		if o.Contains(v) {
			out.Add(v)
		}
	}
	return out
}

// Difference returns a new set holding the elements of s that are not in o.
// Neither input is modified.
func (s *Set[T]) Difference(o *Set[T]) *Set[T] {
	out := &Set[T]{}
	for v := range s.All() {
		if !o.Contains(v) {
			out.Add(v)
		}
	}
	return out
}

// Sorted returns the elements of s as a sorted slice.
//
// It is a function, not a method, because it needs T to be ordered while
// Set only requires comparable: a method cannot tighten its receiver's
// constraint.
func Sorted[T cmp.Ordered](s *Set[T]) []T {
	return slices.Sorted(s.All())
}
