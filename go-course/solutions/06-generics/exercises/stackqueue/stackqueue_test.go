package stackqueue

import (
	"math/rand/v2"
	"testing"
)

// container is the API shared by Stack and Queue. Tests that only care about
// the "empty" behaviour run against both through it.
type container[T any] interface {
	Push(T)
	Pop() (T, bool)
	Peek() (T, bool)
	Len() int
}

// Compile-time checks that both types satisfy the shared interface.
var (
	_ container[int] = (*Stack[int])(nil)
	_ container[int] = (*Queue[int])(nil)
)

func TestEmpty(t *testing.T) {
	for name, c := range map[string]container[string]{
		"Stack": &Stack[string]{},
		"Queue": &Queue[string]{},
	} {
		t.Run(name, func(t *testing.T) {
			if n := c.Len(); n != 0 {
				t.Errorf("empty %s: Len() = %d, want 0", name, n)
			}
			if v, ok := c.Pop(); ok || v != "" {
				t.Errorf("empty %s: Pop() = (%q, %v), want (\"\", false)", name, v, ok)
			}
			if v, ok := c.Peek(); ok || v != "" {
				t.Errorf("empty %s: Peek() = (%q, %v), want (\"\", false)", name, v, ok)
			}
			// Popping an empty container must not break it.
			c.Push("x")
			if v, ok := c.Pop(); !ok || v != "x" {
				t.Errorf("%s after Push(x) on a drained container: Pop() = (%q, %v), want (\"x\", true)", name, v, ok)
			}
		})
	}
}

func TestStackLIFO(t *testing.T) {
	var s Stack[int]
	for i := 1; i <= 3; i++ {
		s.Push(i)
	}
	if s.Len() != 3 {
		t.Fatalf("after 3 pushes: Len() = %d, want 3", s.Len())
	}
	if v, ok := s.Peek(); !ok || v != 3 {
		t.Errorf("Peek() = (%d, %v), want (3, true)", v, ok)
	}
	if s.Len() != 3 {
		t.Errorf("Peek changed Len to %d, want 3", s.Len())
	}
	for _, want := range []int{3, 2, 1} {
		if v, ok := s.Pop(); !ok || v != want {
			t.Errorf("Pop() = (%d, %v), want (%d, true)", v, ok, want)
		}
	}
	if _, ok := s.Pop(); ok {
		t.Error("Pop() on drained stack returned ok = true")
	}
}

func TestQueueFIFO(t *testing.T) {
	var q Queue[string]
	for _, s := range []string{"a", "b", "c"} {
		q.Push(s)
	}
	if v, ok := q.Peek(); !ok || v != "a" {
		t.Errorf("Peek() = (%q, %v), want (\"a\", true)", v, ok)
	}
	if q.Len() != 3 {
		t.Errorf("Peek changed Len to %d, want 3", q.Len())
	}
	for _, want := range []string{"a", "b", "c"} {
		if v, ok := q.Pop(); !ok || v != want {
			t.Errorf("Pop() = (%q, %v), want (%q, true)", v, ok, want)
		}
	}
}

// TestAgainstModel performs a long random sequence of operations on each
// container and on a trivially-correct model, and compares every result.
func TestAgainstModel(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2)) // fixed seed: failures are reproducible

	var s Stack[int]
	var q Queue[int]
	var model []int // stack model: top at the end; queue model: front at 0
	var qmodel []int

	for i := range 10_000 {
		if r.IntN(3) < 2 { // push twice as often as pop so sizes grow
			s.Push(i)
			q.Push(i)
			model = append(model, i)
			qmodel = append(qmodel, i)
			continue
		}
		sv, sok := s.Pop()
		var wantS int
		wantSOK := len(model) > 0
		if wantSOK {
			wantS = model[len(model)-1]
			model = model[:len(model)-1]
		}
		if sv != wantS || sok != wantSOK {
			t.Fatalf("op %d: Stack.Pop() = (%d, %v), want (%d, %v)", i, sv, sok, wantS, wantSOK)
		}

		qv, qok := q.Pop()
		var wantQ int
		wantQOK := len(qmodel) > 0
		if wantQOK {
			wantQ = qmodel[0]
			qmodel = qmodel[1:]
		}
		if qv != wantQ || qok != wantQOK {
			t.Fatalf("op %d: Queue.Pop() = (%d, %v), want (%d, %v)", i, qv, qok, wantQ, wantQOK)
		}
		if s.Len() != len(model) || q.Len() != len(qmodel) {
			t.Fatalf("op %d: Len() = stack %d / queue %d, want %d / %d", i, s.Len(), q.Len(), len(model), len(qmodel))
		}
	}
}

// TestStackPopClearsSlot checks that Pop does not leave a copy of the popped
// value in the backing array, where it would stay reachable (and so never
// garbage collected) until the slot is overwritten.
func TestStackPopClearsSlot(t *testing.T) {
	var s Stack[*[1 << 10]byte]
	s.Push(new([1 << 10]byte))
	s.Push(new([1 << 10]byte))
	s.Pop()
	if s.Len() != 1 || cap(s.items) < 2 {
		t.Fatalf("after 2 pushes and 1 pop: Len() = %d, cap(items) = %d; want 1 and >= 2", s.Len(), cap(s.items))
	}
	backing := s.items[:cap(s.items)]
	if backing[1] != nil {
		t.Error("after Pop, the vacated slot still holds the popped pointer; set it to the zero value before reslicing")
	}
}

func TestZeroValueStructElements(t *testing.T) {
	type job struct {
		ID   int
		Name string
	}
	var q Queue[job]
	q.Push(job{1, "build"})
	q.Push(job{2, "test"})
	if v, _ := q.Pop(); v != (job{1, "build"}) {
		t.Errorf("Queue[job].Pop() = %+v, want {ID:1 Name:build}", v)
	}
}
