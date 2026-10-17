// Package stackqueue provides generic LIFO and FIFO containers.
//
// Run the tests with:
//
//	go test ./modules/06-generics/exercises/stackqueue/
package stackqueue

// Stack is a last-in, first-out container. The zero value is an empty stack
// ready to use.
type Stack[T any] struct {
	items []T // keep this field: the tests inspect it
}

// Push adds v to the top of the stack.
func (s *Stack[T]) Push(v T) {
	// TODO
}

// Pop removes and returns the top element. If the stack is empty it returns
// the zero value of T and false.
//
// Hint: `var zero T` is how you spell "the zero value of whatever T is".
// Think about what is left behind in the backing array after you reslice.
func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	// TODO
	return zero, false
}

// Peek returns the top element without removing it. If the stack is empty it
// returns the zero value of T and false.
func (s *Stack[T]) Peek() (T, bool) {
	var zero T
	// TODO
	return zero, false
}

// Len returns the number of elements on the stack.
func (s *Stack[T]) Len() int {
	// TODO
	return 0
}

// Queue is a first-in, first-out container. The zero value is an empty queue
// ready to use.
//
// A slice is fine for this exercise. You may change the fields.
type Queue[T any] struct {
	items []T
}

// Push adds v to the back of the queue.
func (q *Queue[T]) Push(v T) {
	// TODO
}

// Pop removes and returns the front element. If the queue is empty it returns
// the zero value of T and false.
func (q *Queue[T]) Pop() (T, bool) {
	var zero T
	// TODO
	return zero, false
}

// Peek returns the front element without removing it. If the queue is empty
// it returns the zero value of T and false.
func (q *Queue[T]) Peek() (T, bool) {
	var zero T
	// TODO
	return zero, false
}

// Len returns the number of elements in the queue.
func (q *Queue[T]) Len() int {
	// TODO
	return 0
}
