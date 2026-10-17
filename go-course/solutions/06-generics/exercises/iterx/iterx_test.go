package iterx

import (
	"iter"
	"slices"
	"strconv"
	"testing"
)

// countingSeq yields xs in order and counts how many elements it has handed
// out, so tests can check that adapters are lazy and stop pulling early.
type countingSeq struct {
	xs     []int
	pulled int
}

func (c *countingSeq) seq() iter.Seq[int] {
	return func(yield func(int) bool) {
		for _, x := range c.xs {
			c.pulled++
			if !yield(x) {
				return
			}
		}
	}
}

// firstN calls seq directly (not through range) with a yield function that
// asks to stop after n elements. It returns what it received and fails the
// test if seq ignores the stop request, which in a range loop would be a
// runtime panic.
func firstN[T any](t *testing.T, seq iter.Seq[T], n int) []T {
	t.Helper()
	var got []T
	stopped := false
	seq(func(v T) bool {
		if stopped {
			t.Errorf("sequence called yield(%v) after yield had returned false; an iterator must return as soon as yield returns false", v)
			return false
		}
		got = append(got, v)
		if len(got) >= n {
			stopped = true
			return false
		}
		return true
	})
	return got
}

func isEven(i int) bool { return i%2 == 0 }

func TestNaturals(t *testing.T) {
	got := firstN(t, Naturals(), 5)
	if want := []int{0, 1, 2, 3, 4}; !slices.Equal(got, want) {
		t.Errorf("first 5 of Naturals() = %v, want %v", got, want)
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want []int
	}{
		{"mixed", []int{1, 2, 3, 4, 5, 6}, []int{2, 4, 6}},
		{"none", []int{1, 3}, nil},
		{"empty", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slices.Collect(Filter(slices.Values(tt.in), isEven))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Collect(Filter(%v, isEven)) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}

	t.Run("stops when consumer stops", func(t *testing.T) {
		src := &countingSeq{xs: []int{1, 2, 3, 4, 5, 6, 7, 8}}
		got := firstN(t, Filter(src.seq(), isEven), 2)
		if !slices.Equal(got, []int{2, 4}) {
			t.Errorf("first 2 of Filter(1..8, isEven) = %v, want [2 4]", got)
		}
		if src.pulled != 4 {
			t.Errorf("Filter pulled %d elements from its source to produce [2 4], want 4 (it must stop pulling once the consumer stops)", src.pulled)
		}
	})

	t.Run("works on an infinite source", func(t *testing.T) {
		got := firstN(t, Filter(Naturals(), isEven), 3)
		if !slices.Equal(got, []int{0, 2, 4}) {
			t.Errorf("first 3 of Filter(Naturals(), isEven) = %v, want [0 2 4]", got)
		}
	})
}

func TestMap(t *testing.T) {
	got := slices.Collect(Map(slices.Values([]int{1, 20, 300}), strconv.Itoa))
	if want := []string{"1", "20", "300"}; !slices.Equal(got, want) {
		t.Errorf("Collect(Map([1 20 300], Itoa)) = %q, want %q", got, want)
	}

	if got := slices.Collect(Map(slices.Values([]int(nil)), strconv.Itoa)); len(got) != 0 {
		t.Errorf("Collect(Map(empty, Itoa)) = %q, want empty", got)
	}

	t.Run("is lazy and stops when consumer stops", func(t *testing.T) {
		src := &countingSeq{xs: []int{1, 2, 3, 4, 5}}
		calls := 0
		seq := Map(src.seq(), func(i int) int { calls++; return i * i })
		if calls != 0 || src.pulled != 0 {
			t.Fatalf("Map did work before being ranged over: %d calls to f, %d pulls; it must be lazy", calls, src.pulled)
		}
		got := firstN(t, seq, 2)
		if !slices.Equal(got, []int{1, 4}) {
			t.Errorf("first 2 of Map(1..5, square) = %v, want [1 4]", got)
		}
		if calls != 2 || src.pulled != 2 {
			t.Errorf("after taking 2: f called %d times, source pulled %d times; want 2 and 2", calls, src.pulled)
		}
	})
}

func TestTake(t *testing.T) {
	tests := []struct {
		name       string
		in         []int
		n          int
		want       []int
		wantPulled int
	}{
		{"fewer than available", []int{1, 2, 3, 4, 5}, 3, []int{1, 2, 3}, 3},
		{"exactly available", []int{1, 2}, 2, []int{1, 2}, 2},
		{"more than available", []int{1, 2}, 5, []int{1, 2}, 2},
		{"zero", []int{1, 2}, 0, nil, 0},
		{"negative", []int{1, 2}, -1, nil, 0},
		{"empty source", nil, 3, nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &countingSeq{xs: tt.in}
			got := slices.Collect(Take(src.seq(), tt.n))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Collect(Take(%v, %d)) = %v, want %v", tt.in, tt.n, got, tt.want)
			}
			if src.pulled != tt.wantPulled {
				t.Errorf("Take(%v, %d) pulled %d elements from its source, want %d (don't pull the element after the last one you need)", tt.in, tt.n, src.pulled, tt.wantPulled)
			}
		})
	}

	t.Run("infinite source", func(t *testing.T) {
		got := slices.Collect(Take(Naturals(), 4))
		if !slices.Equal(got, []int{0, 1, 2, 3}) {
			t.Errorf("Collect(Take(Naturals(), 4)) = %v, want [0 1 2 3]", got)
		}
	})

	t.Run("consumer stops before n", func(t *testing.T) {
		got := firstN(t, Take(Naturals(), 10), 2)
		if !slices.Equal(got, []int{0, 1}) {
			t.Errorf("first 2 of Take(Naturals(), 10) = %v, want [0 1]", got)
		}
	})

	t.Run("sequence is reusable", func(t *testing.T) {
		seq := Take(slices.Values([]int{7, 8, 9}), 2)
		first := slices.Collect(seq)
		second := slices.Collect(seq)
		if !slices.Equal(first, []int{7, 8}) || !slices.Equal(second, []int{7, 8}) {
			t.Errorf("ranging over the same Take sequence twice gave %v then %v, want [7 8] both times (is your counter declared outside the returned func?)", first, second)
		}
	})
}

func TestEnumerate(t *testing.T) {
	var idx []int
	var vals []string
	for i, v := range Enumerate(slices.Values([]string{"a", "b", "c"})) {
		idx = append(idx, i)
		vals = append(vals, v)
	}
	if !slices.Equal(idx, []int{0, 1, 2}) || !slices.Equal(vals, []string{"a", "b", "c"}) {
		t.Errorf("Enumerate([a b c]) yielded indexes %v values %q, want [0 1 2] [a b c]", idx, vals)
	}

	// Early stop, calling the Seq2 directly.
	calls := 0
	Enumerate(Naturals())(func(i, v int) bool {
		calls++
		if i != v {
			t.Errorf("Enumerate(Naturals()) yielded (%d, %d), want index == value", i, v)
		}
		return calls < 3
	})
	if calls != 3 {
		t.Errorf("Enumerate called yield %d times; want 3 (it must stop when yield returns false)", calls)
	}

	t.Run("reusable", func(t *testing.T) {
		seq := Enumerate(slices.Values([]int{5, 6}))
		for range 2 {
			var got []int
			for i := range seq {
				got = append(got, i)
			}
			if !slices.Equal(got, []int{0, 1}) {
				t.Errorf("Enumerate indexes = %v, want [0 1] on every pass (does the index restart?)", got)
			}
		}
	})
}

// TestPipeline composes every adapter, as real code would.
func TestPipeline(t *testing.T) {
	// The first five even squares, labelled with their position.
	evens := Filter(Naturals(), isEven)
	squares := Map(evens, func(i int) int { return i * i })
	labelled := Map(Take(squares, 5), func(i int) string { return strconv.Itoa(i) })

	var got []string
	for i, s := range Enumerate(labelled) {
		got = append(got, strconv.Itoa(i)+":"+s)
	}
	want := []string{"0:0", "1:4", "2:16", "3:36", "4:64"}
	if !slices.Equal(got, want) {
		t.Errorf("pipeline = %q, want %q", got, want)
	}
}
