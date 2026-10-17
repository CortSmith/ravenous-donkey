package paginate

import (
	"errors"
	"fmt"
	"testing"
)

func TestPages(t *testing.T) {
	tests := []struct {
		name           string
		total, perPage int
		want           int
	}{
		{"no items", 0, 10, 0},
		{"single partial page", 3, 10, 1},
		{"exactly one page", 10, 10, 1},
		{"exact multiple", 20, 10, 2},
		{"one over a multiple", 21, 10, 3},
		{"one item per page", 7, 1, 7},
		{"negative total", -1, 10, 0},
		{"zero per page", 10, 0, 0},
		{"negative per page", 10, -5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // safe: since Go 1.22 each iteration has its own tt
			if got := Pages(tt.total, tt.perPage); got != tt.want {
				t.Errorf("Pages(%d, %d) = %d, want %d", tt.total, tt.perPage, got, tt.want)
			}
		})
	}
}

func TestBounds(t *testing.T) {
	tests := []struct {
		name                 string
		total, perPage, page int
		wantStart, wantEnd   int
		wantErr              error
	}{
		{name: "first full page", total: 25, perPage: 10, page: 1, wantStart: 0, wantEnd: 10},
		{name: "middle page", total: 25, perPage: 10, page: 2, wantStart: 10, wantEnd: 20},
		{name: "last partial page", total: 25, perPage: 10, page: 3, wantStart: 20, wantEnd: 25},
		{name: "last full page", total: 20, perPage: 10, page: 2, wantStart: 10, wantEnd: 20},
		{name: "fewer items than a page", total: 3, perPage: 10, page: 1, wantStart: 0, wantEnd: 3},
		{name: "empty first page", total: 0, perPage: 10, page: 1, wantStart: 0, wantEnd: 0},
		{name: "page after last", total: 25, perPage: 10, page: 4, wantErr: ErrInvalidPage},
		{name: "page after exact multiple", total: 20, perPage: 10, page: 3, wantErr: ErrInvalidPage},
		{name: "page zero", total: 25, perPage: 10, page: 0, wantErr: ErrInvalidPage},
		{name: "negative page", total: 25, perPage: 10, page: -1, wantErr: ErrInvalidPage},
		{name: "page 2 of empty", total: 0, perPage: 10, page: 2, wantErr: ErrInvalidPage},
		{name: "negative total", total: -1, perPage: 10, page: 1, wantErr: ErrInvalidSize},
		{name: "zero per page", total: 10, perPage: 0, page: 1, wantErr: ErrInvalidSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			start, end, err := Bounds(tt.total, tt.perPage, tt.page)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Bounds(%d, %d, %d) error = %v, want %v",
					tt.total, tt.perPage, tt.page, err, tt.wantErr)
			}
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("Bounds(%d, %d, %d) = [%d, %d), want [%d, %d)",
					tt.total, tt.perPage, tt.page, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// TestBoundsCoverEveryItem checks a property instead of single examples:
// walking pages 1..Pages(total, perPage) must visit every index in
// [0, total) exactly once, in order, and every slice must be in range.
func TestBoundsCoverEveryItem(t *testing.T) {
	for total := 0; total <= 40; total++ {
		for perPage := 1; perPage <= 12; perPage++ {
			next := 0 // the next index we expect a page to start at
			for page := 1; page <= Pages(total, perPage); page++ {
				start, end, err := Bounds(total, perPage, page)
				if err != nil {
					t.Fatalf("Bounds(%d, %d, %d) unexpected error: %v", total, perPage, page, err)
				}
				if start != next || end <= start || end > total {
					t.Fatalf("Bounds(%d, %d, %d) = [%d, %d), want start %d and start < end <= %d",
						total, perPage, page, start, end, next, total)
				}
				next = end
			}
			if next != total {
				t.Fatalf("pages for total=%d perPage=%d covered [0, %d), want [0, %d)",
					total, perPage, next, total)
			}
		}
	}
}

func ExampleBounds() {
	items := []string{"a", "b", "c", "d", "e"}
	for page := 1; page <= Pages(len(items), 2); page++ {
		start, end, _ := Bounds(len(items), 2, page)
		fmt.Println(page, items[start:end])
	}
	// Output:
	// 1 [a b]
	// 2 [c d]
	// 3 [e]
}
