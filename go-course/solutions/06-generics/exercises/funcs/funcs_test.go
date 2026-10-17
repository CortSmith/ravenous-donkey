package funcs

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Names is a named slice type. Filter must hand it back unchanged in type,
// which is what the S ~[]E pattern buys you.
type Names []string

func TestMap(t *testing.T) {
	t.Run("int to string", func(t *testing.T) {
		in := []int{1, -2, 30}
		got := Map(in, strconv.Itoa)
		want := []string{"1", "-2", "30"}
		if !slices.Equal(got, want) {
			t.Errorf("Map(%v, strconv.Itoa) = %q, want %q", in, got, want)
		}
	})

	t.Run("string to length", func(t *testing.T) {
		in := Names{"go", "gopher", ""}
		got := Map(in, func(s string) int { return len(s) })
		want := []int{2, 6, 0}
		if !slices.Equal(got, want) {
			t.Errorf("Map(%q, len) = %v, want %v", in, got, want)
		}
	})

	t.Run("empty and nil input", func(t *testing.T) {
		for _, in := range [][]int{nil, {}} {
			got := Map(in, func(i int) int { return i * 2 })
			if len(got) != 0 {
				t.Errorf("Map(%#v, double) = %v, want empty", in, got)
			}
		}
	})

	t.Run("calls f exactly once per element, in order", func(t *testing.T) {
		in := []string{"a", "b", "c"}
		var calls []string
		Map(in, func(s string) string {
			calls = append(calls, s)
			return strings.ToUpper(s)
		})
		if !slices.Equal(calls, in) {
			t.Errorf("Map called f with %q, want %q (once each, in order)", calls, in)
		}
	})

	t.Run("does not modify input", func(t *testing.T) {
		in := []int{1, 2, 3}
		orig := slices.Clone(in)
		Map(in, func(i int) int { return i * 10 })
		if !slices.Equal(in, orig) {
			t.Errorf("Map modified its input: now %v, was %v", in, orig)
		}
	})
}

func TestFilter(t *testing.T) {
	isEven := func(i int) bool { return i%2 == 0 }

	tests := []struct {
		name string
		in   []int
		want []int
	}{
		{"mixed", []int{1, 2, 3, 4, 5, 6}, []int{2, 4, 6}},
		{"none kept", []int{1, 3, 5}, []int{}},
		{"all kept", []int{2, 4}, []int{2, 4}},
		{"keeps duplicates and order", []int{4, 2, 4, 1, 2}, []int{4, 2, 4, 2}},
		{"empty", []int{}, []int{}},
		{"nil", nil, []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filter(tt.in, isEven)
			// Compare by contents only: whether an empty result is nil is
			// left to the implementation.
			if !slices.Equal(got, tt.want) && !(len(got) == 0 && len(tt.want) == 0) {
				t.Errorf("Filter(%v, isEven) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}

	t.Run("preserves named slice type", func(t *testing.T) {
		in := Names{"ann", "bob", "alice"}
		var got Names = Filter(in, func(s string) bool { return strings.HasPrefix(s, "a") })
		want := Names{"ann", "alice"}
		if !slices.Equal(got, want) {
			t.Errorf("Filter(%q, hasPrefixA) = %q, want %q", in, got, want)
		}
	})

	t.Run("does not modify input or share its backing array", func(t *testing.T) {
		in := []int{1, 2, 3, 4}
		orig := slices.Clone(in)
		got := Filter(in, isEven)
		if !slices.Equal(in, orig) {
			t.Fatalf("Filter modified its input: now %v, was %v (did you filter in place with s[:0]?)", in, orig)
		}
		if len(got) > 0 {
			got[0] = 99
			if !slices.Equal(in, orig) {
				t.Errorf("result of Filter shares memory with its input: writing got[0] changed input to %v", in)
			}
		}
	})
}

func TestReduce(t *testing.T) {
	t.Run("sum", func(t *testing.T) {
		in := []int{1, 2, 3, 4}
		got := Reduce(in, 0, func(acc, v int) int { return acc + v })
		if got != 10 {
			t.Errorf("Reduce(%v, 0, add) = %d, want 10", in, got)
		}
	})

	t.Run("folds left to right", func(t *testing.T) {
		in := []string{"a", "b", "c"}
		got := Reduce(in, ">", func(acc, v string) string { return acc + v })
		if got != ">abc" {
			t.Errorf("Reduce(%q, \">\", concat) = %q, want %q", in, got, ">abc")
		}
	})

	t.Run("accumulator of a different type", func(t *testing.T) {
		in := []string{"go", "is", "fun"}
		got := Reduce(in, 0, func(acc int, s string) int { return acc + len(s) })
		if got != 7 {
			t.Errorf("Reduce(%q, 0, addLen) = %d, want 7", in, got)
		}
	})

	t.Run("builds a map", func(t *testing.T) {
		in := []string{"x", "y", "x"}
		got := Reduce(in, map[string]int{}, func(m map[string]int, s string) map[string]int {
			m[s]++
			return m
		})
		if got["x"] != 2 || got["y"] != 1 || len(got) != 2 {
			t.Errorf("Reduce(%q, {}, count) = %v, want map[x:2 y:1]", in, got)
		}
	})

	t.Run("empty returns init", func(t *testing.T) {
		got := Reduce([]int(nil), 42, func(acc, v int) int { return acc + v })
		if got != 42 {
			t.Errorf("Reduce(nil, 42, add) = %d, want 42", got)
		}
	})
}

func TestGroupBy(t *testing.T) {
	t.Run("by length keeps order within groups", func(t *testing.T) {
		in := Names{"go", "rust", "c", "zig", "java", "d", "ocaml"}
		got := GroupBy(in, func(s string) int { return len(s) })
		want := map[int]Names{
			1: {"c", "d"},
			2: {"go"},
			3: {"zig"},
			4: {"rust", "java"},
			5: {"ocaml"},
		}
		if len(got) != len(want) {
			t.Fatalf("GroupBy(%q, len) has %d groups %v, want %d groups %v", in, len(got), got, len(want), want)
		}
		for k, w := range want {
			if g := got[k]; !slices.Equal(g, w) {
				t.Errorf("GroupBy(%q, len)[%d] = %q, want %q", in, k, g, w)
			}
		}
	})

	t.Run("does not modify input", func(t *testing.T) {
		in := []int{3, 1, 4, 1, 5, 9, 2, 6}
		orig := slices.Clone(in)
		GroupBy(in, func(i int) bool { return i%2 == 0 })
		if !slices.Equal(in, orig) {
			t.Errorf("GroupBy modified its input: now %v, was %v", in, orig)
		}
	})

	t.Run("empty input gives empty non-nil map", func(t *testing.T) {
		got := GroupBy([]int(nil), func(i int) int { return i })
		if got == nil {
			t.Fatal("GroupBy(nil, id) returned a nil map, want an empty non-nil map")
		}
		if len(got) != 0 {
			t.Errorf("GroupBy(nil, id) = %v, want empty map", got)
		}
	})
}
