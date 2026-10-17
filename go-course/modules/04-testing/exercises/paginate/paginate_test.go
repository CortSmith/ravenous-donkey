package paginate

import "testing"

// Write tests to expose the 3 bugs in paginate.go.
//
// TODO: Write TestPages (table-driven) with:
//   - Edge cases: 0 items, exact multiples (20 items, 10 per page),
//     one-over multiples, invalid inputs (negative, zero per page)
//
// TODO: Write TestBounds (table-driven) with:
//   - First page, middle page, last page (partial and exact)
//   - Error cases: page 0, page too high, negative total, invalid perPage
//   - Use errors.Is to check error types
//
// TODO: Bonus TestBoundsCoverEveryItem (property test):
//   - Iterate total=0..40, perPage=1..12
//   - For each pair, walk pages 1..Pages(total,perPage)
//   - Verify pages cover [0,total) exactly once with no gaps
//
// TODO: Bonus ExampleBounds showing typical usage

func TestPagesSimple(t *testing.T) {
	got := Pages(25, 10)
	if got == 0 {
		t.Errorf("Pages(25, 10) = 0, expected non-zero")
	}
}

func TestBoundsSimple(t *testing.T) {
	start, end, err := Bounds(25, 10, 1)
	if err != nil {
		t.Fatalf("Bounds(25, 10, 1) error = %v", err)
	}
	if start != 0 {
		t.Errorf("Bounds(25, 10, 1) start = %d, want 0", start)
	}
	if end <= start {
		t.Errorf("Bounds(25, 10, 1) end = %d not > start %d", end, start)
	}
}
