package set

import (
	"slices"
	"testing"
)

// contents returns the sorted elements of s, using only the All iterator.
func contents(t *testing.T, s *Set[int]) []int {
	t.Helper()
	var got []int
	for v := range s.All() {
		got = append(got, v)
	}
	slices.Sort(got)
	return got
}

func assertSet(t *testing.T, name string, s *Set[int], want ...int) {
	t.Helper()
	got := contents(t, s)
	if !slices.Equal(got, want) {
		t.Errorf("%s: elements = %v, want %v", name, got, want)
	}
	if s.Len() != len(want) {
		t.Errorf("%s: Len() = %d, want %d", name, s.Len(), len(want))
	}
	for _, v := range want {
		if !s.Contains(v) {
			t.Errorf("%s: Contains(%d) = false, want true", name, v)
		}
	}
}

func TestZeroValueIsUsable(t *testing.T) {
	var s Set[string]
	if s.Len() != 0 {
		t.Errorf("zero Set: Len() = %d, want 0", s.Len())
	}
	if s.Contains("x") {
		t.Error(`zero Set: Contains("x") = true, want false`)
	}
	if s.Remove("x") {
		t.Error(`zero Set: Remove("x") = true, want false`)
	}
	if !s.Add("x") {
		t.Error(`zero Set: Add("x") = false, want true (did you forget to initialise the map?)`)
	}
	if !s.Contains("x") || s.Len() != 1 {
		t.Errorf(`after Add("x"): Contains = %v, Len = %d; want true, 1`, s.Contains("x"), s.Len())
	}
}

func TestOf(t *testing.T) {
	assertSet(t, "Of(3, 1, 2, 3, 1)", Of(3, 1, 2, 3, 1), 1, 2, 3)
	assertSet(t, "Of()", Of[int]())
}

func TestAddRemove(t *testing.T) {
	s := Of[int]()
	steps := []struct {
		op   string
		v    int
		want bool
	}{
		{"add", 1, true},
		{"add", 1, false},
		{"add", 2, true},
		{"remove", 1, true},
		{"remove", 1, false},
		{"remove", 42, false},
		{"add", 1, true},
	}
	for i, st := range steps {
		var got bool
		switch st.op {
		case "add":
			got = s.Add(st.v)
		case "remove":
			got = s.Remove(st.v)
		}
		if got != st.want {
			t.Errorf("step %d: %s(%d) = %v, want %v", i, st.op, st.v, got, st.want)
		}
	}
	assertSet(t, "after steps", s, 1, 2)
}

func TestNilSetReads(t *testing.T) {
	var s *Set[int]
	if s.Len() != 0 {
		t.Errorf("nil *Set: Len() = %d, want 0", s.Len())
	}
	if s.Contains(1) {
		t.Error("nil *Set: Contains(1) = true, want false")
	}
	for v := range s.All() {
		t.Errorf("nil *Set: All() yielded %d, want nothing", v)
	}
}

func TestSetAlgebra(t *testing.T) {
	a := func() *Set[int] { return Of(1, 2, 3, 4) }
	b := func() *Set[int] { return Of(3, 4, 5) }
	var nilSet *Set[int]

	tests := []struct {
		name string
		got  *Set[int]
		want []int
	}{
		{"a ∪ b", a().Union(b()), []int{1, 2, 3, 4, 5}},
		{"a ∩ b", a().Intersect(b()), []int{3, 4}},
		{"b ∩ a", b().Intersect(a()), []int{3, 4}},
		{"a − b", a().Difference(b()), []int{1, 2}},
		{"b − a", b().Difference(a()), []int{5}},
		{"a ∪ ∅", a().Union(Of[int]()), []int{1, 2, 3, 4}},
		{"a ∩ ∅", a().Intersect(Of[int]()), nil},
		{"a ∪ nil", a().Union(nilSet), []int{1, 2, 3, 4}},
		{"a ∩ nil", a().Intersect(nilSet), nil},
		{"a − nil", a().Difference(nilSet), []int{1, 2, 3, 4}},
		{"nil ∪ b", nilSet.Union(b()), []int{3, 4, 5}},
		{"nil ∩ b", nilSet.Intersect(b()), nil},
		{"nil − b", nilSet.Difference(b()), nil},
		{"disjoint ∩", Of(1, 2).Intersect(Of(3, 4)), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got == nil {
				t.Fatalf("%s returned a nil *Set, want a (possibly empty) set", tt.name)
			}
			assertSet(t, tt.name, tt.got, tt.want...)
		})
	}
}

func TestAlgebraDoesNotModifyInputs(t *testing.T) {
	a, b := Of(1, 2, 3), Of(2, 3, 4)
	u := a.Union(b)
	a.Intersect(b)
	a.Difference(b)
	assertSet(t, "a after ops", a, 1, 2, 3)
	assertSet(t, "b after ops", b, 2, 3, 4)

	// The result must be independent storage, not an alias of an input.
	u.Add(100)
	if a.Contains(100) || b.Contains(100) {
		t.Error("adding to the result of Union also changed an input: the result must not share the input's map")
	}
}

func TestAllStopsWhenYieldReturnsFalse(t *testing.T) {
	s := Of(1, 2, 3, 4, 5)
	calls := 0
	s.All()(func(int) bool {
		calls++
		return false // the consumer says "stop" after the first element
	})
	if calls != 1 {
		t.Errorf("All() called yield %d times after yield returned false on the first call, want 1", calls)
	}

	// The same thing via range-over-func: break after two elements.
	n := 0
	for range s.All() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("ranging over All() with break after 2: saw %d elements, want 2", n)
	}
}

func TestSorted(t *testing.T) {
	got := Sorted(Of("pear", "apple", "fig", "apple"))
	want := []string{"apple", "fig", "pear"}
	if !slices.Equal(got, want) {
		t.Errorf("Sorted(Of(pear, apple, fig, apple)) = %q, want %q", got, want)
	}
	if got := Sorted(Of[float64]()); len(got) != 0 {
		t.Errorf("Sorted(empty) = %v, want empty", got)
	}
}

func TestStructElements(t *testing.T) {
	type point struct{ X, Y int }
	s := Of(point{1, 2}, point{1, 2}, point{2, 1})
	if s.Len() != 2 {
		t.Errorf("Of(p12, p12, p21).Len() = %d, want 2", s.Len())
	}
	if !s.Contains(point{2, 1}) {
		t.Error("Contains(point{2, 1}) = false, want true")
	}
}
