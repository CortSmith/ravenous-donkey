package stack

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// The method set of *Stack must include everything; the method set of the
// value type Stack must be EMPTY, i.e. every method has a pointer receiver.
var _ fmt.Stringer = (*Stack)(nil)

func TestReceiversArePointers(t *testing.T) {
	valueType := reflect.TypeOf(Stack{})
	if n := valueType.NumMethod(); n != 0 {
		var names []string
		for i := range n {
			names = append(names, valueType.Method(i).Name)
		}
		t.Errorf("Stack (value type) has methods %v; every method should use a pointer receiver (*Stack)", names)
	}
	ptrType := reflect.TypeOf(&Stack{})
	for _, name := range []string{"Push", "Pop", "Peek", "Len", "Values", "Clone", "String"} {
		if _, ok := ptrType.MethodByName(name); !ok {
			t.Errorf("*Stack is missing method %s", name)
		}
	}
}

func TestZeroValueIsUsable(t *testing.T) {
	var s Stack
	if got := s.Len(); got != 0 {
		t.Errorf("zero Stack: Len() = %d, want 0", got)
	}
	if v, ok := s.Pop(); ok || v != 0 {
		t.Errorf("zero Stack: Pop() = %d, %v; want 0, false", v, ok)
	}
	if v, ok := s.Peek(); ok || v != 0 {
		t.Errorf("zero Stack: Peek() = %d, %v; want 0, false", v, ok)
	}
	s.Push(7)
	if got := s.Len(); got != 1 {
		t.Fatalf("after Push(7): Len() = %d, want 1 (did Push mutate a copy? check the receiver)", got)
	}
}

func TestPushPopOrder(t *testing.T) {
	var s Stack
	for i := 1; i <= 5; i++ {
		s.Push(i * 10)
	}
	if got := s.Len(); got != 5 {
		t.Fatalf("Len() = %d after 5 pushes, want 5", got)
	}
	for want := 50; want >= 10; want -= 10 {
		if v, ok := s.Peek(); !ok || v != want {
			t.Fatalf("Peek() = %d, %v; want %d, true", v, ok, want)
		}
		if v, ok := s.Pop(); !ok || v != want {
			t.Fatalf("Pop() = %d, %v; want %d, true", v, ok, want)
		}
	}
	if v, ok := s.Pop(); ok {
		t.Errorf("Pop() on drained stack = %d, true; want 0, false", v)
	}
	if got := s.Len(); got != 0 {
		t.Errorf("Len() on drained stack = %d, want 0", got)
	}
}

func TestPeekDoesNotRemove(t *testing.T) {
	var s Stack
	s.Push(1)
	s.Push(2)
	for range 3 {
		if v, ok := s.Peek(); !ok || v != 2 {
			t.Fatalf("Peek() = %d, %v; want 2, true", v, ok)
		}
	}
	if got := s.Len(); got != 2 {
		t.Errorf("Len() after Peeks = %d, want 2", got)
	}
}

func TestInterleaved(t *testing.T) {
	var s Stack
	s.Push(1)
	s.Push(2)
	s.Pop()
	s.Push(3)
	s.Push(4)
	s.Pop()
	s.Push(5)
	if got, want := s.Values(), []int{1, 3, 5}; !slices.Equal(got, want) {
		t.Errorf("Values() = %v, want %v", got, want)
	}
}

func TestValuesIsACopy(t *testing.T) {
	var s Stack
	s.Push(1)
	s.Push(2)
	s.Push(3)
	vals := s.Values()
	if want := []int{1, 2, 3}; !slices.Equal(vals, want) {
		t.Fatalf("Values() = %v, want %v (bottom first)", vals, want)
	}
	vals[0] = 99
	if got := s.Values(); got[0] != 1 {
		t.Errorf("mutating the slice returned by Values changed the stack: Values() = %v", got)
	}

	// Appending to the returned slice must not be visible either.
	s.Pop() // len 2, cap >= 3: an aliased slice would expose the old slot
	vals = s.Values()
	_ = append(vals, 42)
	s.Push(3)
	if v, _ := s.Peek(); v != 3 {
		t.Errorf("append to Values() result clobbered the stack: Peek() = %d, want 3", v)
	}

	var empty Stack
	if got := empty.Values(); len(got) != 0 {
		t.Errorf("empty Stack Values() = %v, want empty", got)
	}
}

func TestCloneIsIndependent(t *testing.T) {
	var s Stack
	s.Push(1)
	s.Push(2)
	c := s.Clone()
	if c == &s {
		t.Fatal("Clone() returned the same pointer")
	}
	c.Push(3)
	s.Pop()
	s.Push(20)
	if got, want := s.Values(), []int{1, 20}; !slices.Equal(got, want) {
		t.Errorf("original after edits = %v, want %v", got, want)
	}
	if got, want := c.Values(), []int{1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("clone after edits = %v, want %v (do they share a backing array?)", got, want)
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		push []int
		want string
	}{
		{nil, "Stack[]"},
		{[]int{1}, "Stack[1]"},
		{[]int{1, 2, 3}, "Stack[1 2 3]"},
		{[]int{-4, 0}, "Stack[-4 0]"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			s := &Stack{}
			for _, v := range tt.push {
				s.Push(v)
			}
			if got := s.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			if got := fmt.Sprint(s); got != tt.want {
				t.Errorf("fmt.Sprint(stack) = %q, want %q", got, tt.want)
			}
		})
	}
}
