package stackqueue

// RingQueue is a FIFO queue backed by a circular buffer whose size is a power
// of two. Popping never shrinks the buffer and pushing only reallocates when
// the buffer is genuinely full, so a queue in steady state (as many pushes as
// pops) allocates nothing. Queue, by contrast, keeps sliding its window
// forward with q.items[1:], which eventually exhausts the capacity and forces
// append to copy everything into a new array.
//
// The zero value is an empty queue ready to use.
type RingQueue[T any] struct {
	buf  []T
	head int // index of the front element
	n    int // number of elements
}

// Push adds v to the back of the queue.
func (q *RingQueue[T]) Push(v T) {
	if q.n == len(q.buf) {
		q.grow()
	}
	q.buf[(q.head+q.n)&(len(q.buf)-1)] = v // & works because len is a power of two
	q.n++
}

func (q *RingQueue[T]) grow() {
	size := max(8, 2*len(q.buf))
	buf := make([]T, size)
	// Unroll the ring into the new buffer, front first.
	k := copy(buf, q.buf[q.head:])
	copy(buf[k:], q.buf[:q.head])
	q.buf, q.head = buf, 0
}

// Pop removes and returns the front element. If the queue is empty it returns
// the zero value of T and false.
func (q *RingQueue[T]) Pop() (T, bool) {
	var zero T
	if q.n == 0 {
		return zero, false
	}
	v := q.buf[q.head]
	q.buf[q.head] = zero
	q.head = (q.head + 1) & (len(q.buf) - 1)
	q.n--
	return v, true
}

// Peek returns the front element without removing it.
func (q *RingQueue[T]) Peek() (T, bool) {
	if q.n == 0 {
		var zero T
		return zero, false
	}
	return q.buf[q.head], true
}

// Len returns the number of elements in the queue.
func (q *RingQueue[T]) Len() int { return q.n }
