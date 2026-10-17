// Package stack implements a LIFO stack of ints whose zero value is ready
// to use.
package stack

import "fmt"

// Stack is a last-in, first-out stack of ints. The zero value is an empty
// stack ready to use.
//
// TODO: this starter "works" on paper but every test fails. Read the
// method declarations carefully: what does each method operate on? Fix the
// receivers (all of them; don't mix receiver kinds), then fill in the
// remaining TODOs.
type Stack struct {
	items []int
}

// Push adds v to the top of the stack.
func (s Stack) Push(v int) {
	s.items = append(s.items, v)
}

// Pop removes and returns the top element. ok is false if the stack is
// empty.
func (s Stack) Pop() (v int, ok bool) {
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
func (s Stack) Peek() (v int, ok bool) {
	return 0, false // TODO
}

// Len reports the number of elements on the stack.
func (s Stack) Len() int { return len(s.items) }

// Values returns a copy of the elements, bottom first. Mutating (or
// appending to) the result must not affect the stack.
func (s Stack) Values() []int {
	return s.items // TODO: this leaks the internal slice
}

// Clone returns an independent copy of the stack.
func (s Stack) Clone() *Stack {
	return &s // TODO: is this really independent?
}

// String formats the stack bottom-to-top, e.g. "Stack[1 2 3]"; an empty
// stack is "Stack[]".
func (s Stack) String() string {
	return fmt.Sprint(s.items) // TODO
}
