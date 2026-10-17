// Package stack implements a LIFO stack of ints whose zero value is ready
// to use.
package stack

import (
	"fmt"
	"slices"
)

// Stack is a last-in, first-out stack of ints. The zero value is an empty
// stack ready to use. Every method has a pointer receiver because Push and
// Pop mutate the stack, and a type's methods should not mix receiver kinds.
// A Stack must not be copied after first use (copies share a backing array);
// use Clone instead.
type Stack struct {
	items []int
}

// Push adds v to the top of the stack.
func (s *Stack) Push(v int) {
	s.items = append(s.items, v)
}

// Pop removes and returns the top element. ok is false if the stack is
// empty.
func (s *Stack) Pop() (v int, ok bool) {
	if len(s.items) == 0 {
		return 0, false
	}
	last := len(s.items) - 1
	v = s.items[last]
	s.items = s.items[:last]
	return v, true
}

// Peek returns the top element without removing it. ok is false if the
// stack is empty.
func (s *Stack) Peek() (v int, ok bool) {
	if len(s.items) == 0 {
		return 0, false
	}
	return s.items[len(s.items)-1], true
}

// Len reports the number of elements on the stack.
func (s *Stack) Len() int { return len(s.items) }

// Values returns a copy of the elements, bottom first. Mutating the result
// does not affect the stack.
func (s *Stack) Values() []int { return slices.Clone(s.items) }

// Clone returns an independent copy of the stack.
func (s *Stack) Clone() *Stack { return &Stack{items: slices.Clone(s.items)} }

// String formats the stack bottom-to-top, e.g. "Stack[1 2 3]".
func (s *Stack) String() string { return fmt.Sprintf("Stack%v", s.items) }
