package dedupe

import (
	"slices"
	"testing"
)

func TestDedupe(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil", nil, []string{}},
		{"empty", []string{}, []string{}},
		{"single", []string{"a"}, []string{"a"}},
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"all duplicates", []string{"x", "x", "x", "x"}, []string{"x"}},
		{"keeps first occurrence order", []string{"b", "a", "b", "c", "a"}, []string{"b", "a", "c"}},
		{"adjacent and distant", []string{"go", "go", "rust", "go", "zig", "rust"}, []string{"go", "rust", "zig"}},
		{"case sensitive", []string{"Go", "go", "GO", "go"}, []string{"Go", "go", "GO"}},
		{"empty string is a value", []string{"", "a", "", "b"}, []string{"", "a", "b"}},
		{"unicode", []string{"café", "cafe", "café"}, []string{"café", "cafe"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := slices.Clone(tt.in)
			got := Dedupe(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Dedupe(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if got == nil {
				t.Errorf("Dedupe(%q) returned nil, want an empty non-nil slice", tt.in)
			}
			if !slices.Equal(tt.in, before) {
				t.Errorf("Dedupe modified its input: was %q, now %q", before, tt.in)
			}
		})
	}
}

// TestDedupeDoesNotAlias checks the result has its own backing array. A
// solution that returns s itself (or s[:k]) when there are no duplicates
// would let callers corrupt each other's data.
func TestDedupeDoesNotAlias(t *testing.T) {
	in := []string{"a", "b", "c"} // no duplicates: the tempting shortcut case
	in = slices.Grow(in, 10)      // spare capacity makes aliasing bugs visible
	out := Dedupe(in)

	out[0] = "CHANGED"
	if in[0] != "a" {
		t.Fatalf("writing to the result changed the input (in[0] = %q); the result must not share memory with the input", in[0])
	}
	_ = append(out, "appended")
	if extended := in[:4]; extended[3] == "appended" {
		t.Fatalf("appending to the result wrote into the input's backing array")
	}
}

func TestCompactInPlace(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want []int
	}{
		{"nil", nil, nil},
		{"empty", []int{}, []int{}},
		{"single", []int{7}, []int{7}},
		{"sorted with runs", []int{1, 1, 2, 3, 3, 3, 4}, []int{1, 2, 3, 4}},
		{"all equal", []int{5, 5, 5}, []int{5}},
		{"no duplicates", []int{1, 2, 3}, []int{1, 2, 3}},
		{"only consecutive duplicates removed", []int{1, 2, 1, 1, 2}, []int{1, 2, 1, 2}},
		{"negative and zero", []int{-3, -3, 0, 0, 0, 2}, []int{-3, 0, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := slices.Clone(tt.in)
			got := CompactInPlace(in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("CompactInPlace(%v) = %v, want %v", tt.in, got, tt.want)
			}
			if len(got) > 0 && &got[0] != &in[0] {
				t.Errorf("CompactInPlace(%v) returned a new slice; it must work in place (reuse the input's backing array)", tt.in)
			}
		})
	}
}

func TestCompactInPlaceAllocations(t *testing.T) {
	data := []int{1, 1, 2, 2, 3, 3, 4, 4, 5, 5}
	allocs := testing.AllocsPerRun(100, func() {
		CompactInPlace(data)
	})
	if allocs != 0 {
		t.Errorf("CompactInPlace allocated %.0f times per call, want 0 (it should work in place)", allocs)
	}
}
