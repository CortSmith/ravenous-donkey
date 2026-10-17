// Package stackqueue provides generic LIFO and FIFO containers.
package stackqueue

// Stack is a last-in, first-out container. The zero value is an empty stack
// ready to use.
type Stack[T any] struct {
	items []T // keep this field: the tests inspect it
}

// Push adds v to the top of the stack.
func (s *Stack[T]) Push(v T) {
	s.items = append(s.items, v)
}

// Pop removes and returns the top element. If the stack is empty it returns
// the zero value of T and false.
func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	last := len(s.items) - 1
	v := s.items[last]
	s.items[last] = zero // drop the reference so the GC can reclaim it
	s.items = s.items[:last]
	return v, true
}

// Peek returns the top element without removing it. If the stack is empty it
// returns the zero value of T and false.
func (s *Stack[T]) Peek() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	return s.items[len(s.items)-1], true
}

// Len returns the number of elements on the stack.
func (s *Stack[T]) Len() int { return len(s.items) }

// Queue is a first-in, first-out container. The zero value is an empty queue
// ready to use.
type Queue[T any] struct {
	items []T
}

// Push adds v to the back of the queue.
func (q *Queue[T]) Push(v T) {
	q.items = append(q.items, v)
}

// Pop removes and returns the front element. If the queue is empty it returns
// the zero value of T and false.
func (q *Queue[T]) Pop() (T, bool) {
	var zero T
	if len(q.items) == 0 {
		return zero, false
	}
	v := q.items[0]
	q.items[0] = zero // the slot stays in the backing array; don't pin v
	q.items = q.items[1:]
	return v, true
}

// Peek returns the front element without removing it. If the queue is empty
// it returns the zero value of T and false.
func (q *Queue[T]) Peek() (T, bool) {
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	return q.items[0], true
}

// Len returns the number of elements in the queue.
func (q *Queue[T]) Len() int { return len(q.items) }
