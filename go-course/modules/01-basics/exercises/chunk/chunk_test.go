package chunk

import (
	"fmt"
	"slices"
	"testing"
)

func TestChunk(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		size int
		want [][]int
	}{
		{"nil input", nil, 3, nil},
		{"empty input", []int{}, 3, nil},
		{"size zero", []int{1, 2, 3}, 0, nil},
		{"size negative", []int{1, 2, 3}, -2, nil},
		{"exact multiple", []int{1, 2, 3, 4}, 2, [][]int{{1, 2}, {3, 4}}},
		{"remainder", []int{1, 2, 3, 4, 5}, 2, [][]int{{1, 2}, {3, 4}, {5}}},
		{"size one", []int{1, 2, 3}, 1, [][]int{{1}, {2}, {3}}},
		{"size equals len", []int{1, 2, 3}, 3, [][]int{{1, 2, 3}}},
		{"size larger than len", []int{1, 2}, 10, [][]int{{1, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Chunk(tt.in, tt.size)
			if tt.want == nil {
				if got != nil {
					t.Errorf("Chunk(%v, %d) = %v, want nil", tt.in, tt.size, got)
				}
				return
			}
			if !slices.EqualFunc(got, tt.want, slices.Equal[[]int]) {
				t.Errorf("Chunk(%v, %d) = %v, want %v", tt.in, tt.size, got, tt.want)
			}
			for i, c := range got {
				if cap(c) != len(c) {
					t.Errorf("Chunk(%v, %d): chunk %d has len %d but cap %d; cap must equal len (use a full slice expression s[lo:hi:hi])",
						tt.in, tt.size, i, len(c), cap(c))
				}
			}
		})
	}
}

func TestChunkIsAViewNotACopy(t *testing.T) {
	s := []int{1, 2, 3, 4, 5}
	chunks := Chunk(s, 2)
	if len(chunks) != 3 {
		t.Fatalf("Chunk(%v, 2) returned %d chunks, want 3", s, len(chunks))
	}
	chunks[1][0] = 30
	if s[2] != 30 {
		t.Errorf("after chunks[1][0] = 30, s[2] = %d, want 30: chunks must share memory with s (no copying)", s[2])
	}
}

func TestChunkAppendDoesNotClobberNeighbour(t *testing.T) {
	s := []int{1, 2, 3, 4, 5, 6}
	chunks := Chunk(s, 2)
	grown := append(chunks[0], 99)

	if s[2] != 3 {
		t.Errorf("append(chunks[0], 99) overwrote s[2]: s = %v, want s[2] == 3", s)
	}
	if chunks[1][0] != 3 {
		t.Errorf("append(chunks[0], 99) overwrote chunks[1][0] = %d, want 3", chunks[1][0])
	}
	if want := []int{1, 2, 99}; !slices.Equal(grown, want) {
		t.Errorf("append(chunks[0], 99) = %v, want %v", grown, want)
	}
}

func TestRotateLeft(t *testing.T) {
	base := []int{1, 2, 3, 4, 5}
	tests := []struct {
		in   []int
		k    int
		want []int
	}{
		{nil, 3, nil},
		{[]int{}, 1, []int{}},
		{[]int{7}, 5, []int{7}},
		{base, 0, []int{1, 2, 3, 4, 5}},
		{base, 1, []int{2, 3, 4, 5, 1}},
		{base, 2, []int{3, 4, 5, 1, 2}},
		{base, 4, []int{5, 1, 2, 3, 4}},
		{base, 5, []int{1, 2, 3, 4, 5}},
		{base, 7, []int{3, 4, 5, 1, 2}},
		{base, 1_000_001, []int{2, 3, 4, 5, 1}},
		{base, -1, []int{5, 1, 2, 3, 4}},
		{base, -5, []int{1, 2, 3, 4, 5}},
		{base, -12, []int{4, 5, 1, 2, 3}},
		{[]int{1, 2}, 1, []int{2, 1}},
		{[]int{1, 2, 3, 4, 5, 6}, 3, []int{4, 5, 6, 1, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v_by_%d", tt.in, tt.k), func(t *testing.T) {
			s := slices.Clone(tt.in)
			RotateLeft(s, tt.k)
			if !slices.Equal(s, tt.want) {
				t.Errorf("RotateLeft(%v, %d) left %v, want %v", tt.in, tt.k, s, tt.want)
			}
		})
	}
}

func TestRotateLeftOnSubslice(t *testing.T) {
	// Rotating a window must not touch elements outside it.
	s := []int{0, 1, 2, 3, 4, 5, 6}
	RotateLeft(s[2:5], 1)
	if want := []int{0, 1, 3, 4, 2, 5, 6}; !slices.Equal(s, want) {
		t.Errorf("after RotateLeft(s[2:5], 1), s = %v, want %v", s, want)
	}
}
