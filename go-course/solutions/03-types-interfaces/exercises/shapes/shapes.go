// Package shapes models simple 2-D shapes behind a small interface.
package shapes

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
)

// Shape is anything with an area and a perimeter.
type Shape interface {
	Area() float64
	Perimeter() float64
}

// Compile-time proof that every concrete type satisfies Shape and
// fmt.Stringer. If a method is renamed or gets the wrong receiver, the
// build breaks here instead of at some distant call site.
var (
	_ Shape        = Circle{}
	_ Shape        = Rect{}
	_ Shape        = Triangle{}
	_ fmt.Stringer = Circle{}
	_ fmt.Stringer = Rect{}
	_ fmt.Stringer = Triangle{}
)

// ErrInvalidTriangle is returned by NewTriangle when the sides cannot form
// a triangle.
var ErrInvalidTriangle = errors.New("invalid triangle")

// Circle is a circle with the given radius.
type Circle struct {
	Radius float64
}

// Area returns πr².
func (c Circle) Area() float64 { return math.Pi * c.Radius * c.Radius }

// Perimeter returns the circumference 2πr.
func (c Circle) Perimeter() float64 { return 2 * math.Pi * c.Radius }

// String formats the circle as "Circle(r=2)".
func (c Circle) String() string { return "Circle(r=" + ftoa(c.Radius) + ")" }

// Rect is an axis-aligned rectangle.
type Rect struct {
	Width, Height float64
}

// Area returns width × height.
func (r Rect) Area() float64 { return r.Width * r.Height }

// Perimeter returns 2 × (width + height).
func (r Rect) Perimeter() float64 { return 2 * (r.Width + r.Height) }

// String formats the rectangle as "Rect(3x4)".
func (r Rect) String() string { return "Rect(" + ftoa(r.Width) + "x" + ftoa(r.Height) + ")" }

// Triangle is a triangle described by its three side lengths.
// Construct it with NewTriangle so the sides are validated.
type Triangle struct {
	A, B, C float64
}

// NewTriangle returns a Triangle with sides a, b and c, or an error wrapping
// ErrInvalidTriangle if any side is not positive or the triangle
// inequality does not hold strictly (degenerate triangles are rejected).
func NewTriangle(a, b, c float64) (Triangle, error) {
	if a <= 0 || b <= 0 || c <= 0 {
		return Triangle{}, fmt.Errorf("sides %g, %g, %g must be positive: %w", a, b, c, ErrInvalidTriangle)
	}
	if a+b <= c || a+c <= b || b+c <= a {
		return Triangle{}, fmt.Errorf("sides %g, %g, %g violate the triangle inequality: %w", a, b, c, ErrInvalidTriangle)
	}
	return Triangle{A: a, B: b, C: c}, nil
}

// Area uses Heron's formula.
func (t Triangle) Area() float64 {
	s := t.Perimeter() / 2
	return math.Sqrt(s * (s - t.A) * (s - t.B) * (s - t.C))
}

// Perimeter returns the sum of the sides.
func (t Triangle) Perimeter() float64 { return t.A + t.B + t.C }

// String formats the triangle as "Triangle(3, 4, 5)".
func (t Triangle) String() string {
	return "Triangle(" + ftoa(t.A) + ", " + ftoa(t.B) + ", " + ftoa(t.C) + ")"
}

// ftoa formats f in the shortest form that round-trips, like %g.
func ftoa(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

// TotalArea returns the sum of the areas of shapes. It returns 0 for an
// empty or nil slice.
func TotalArea(shapes []Shape) float64 {
	var total float64
	for _, s := range shapes {
		total += s.Area()
	}
	return total
}

// Largest returns the shape with the greatest area. If several shapes tie,
// the first one wins. ok is false when shapes is empty.
func Largest(shapes []Shape) (largest Shape, ok bool) {
	if len(shapes) == 0 {
		return nil, false
	}
	largest = shapes[0]
	for _, s := range shapes[1:] {
		if s.Area() > largest.Area() {
			largest = s
		}
	}
	return largest, true
}

// SortByArea sorts shapes in place by ascending area. The sort is stable:
// shapes with equal area keep their original relative order.
func SortByArea(shapes []Shape) {
	slices.SortStableFunc(shapes, func(a, b Shape) int {
		return cmp.Compare(a.Area(), b.Area())
	})
}
