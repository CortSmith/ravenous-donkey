package shapes

import (
	"cmp"
	"slices"
)

// LargestFast is Largest, but it remembers the best area instead of
// recomputing largest.Area() on every comparison (halving the number of
// dynamic Area calls; for triangles each call is a math.Sqrt).
func LargestFast(shapes []Shape) (Shape, bool) {
	if len(shapes) == 0 {
		return nil, false
	}
	best, bestArea := shapes[0], shapes[0].Area()
	for _, s := range shapes[1:] {
		if a := s.Area(); a > bestArea {
			best, bestArea = s, a
		}
	}
	return best, true
}

// keyed pairs a shape with its precomputed area.
type keyed struct {
	area  float64
	shape Shape
}

// SortByAreaFast is SortByArea using the "decorate-sort-undecorate"
// (Schwartzian transform) technique: each Area is computed exactly once
// (n calls instead of ~2·n·log n), and the comparator works on plain
// float64s instead of making two dynamic interface calls per comparison.
// It costs one allocation of n keyed values. Stability is preserved because
// SortStableFunc is stable.
func SortByAreaFast(shapes []Shape) {
	if len(shapes) < 2 {
		return
	}
	tmp := make([]keyed, len(shapes))
	for i, s := range shapes {
		tmp[i] = keyed{area: s.Area(), shape: s}
	}
	slices.SortStableFunc(tmp, func(a, b keyed) int { return cmp.Compare(a.area, b.area) })
	for i := range tmp {
		shapes[i] = tmp[i].shape
	}
}
