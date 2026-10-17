// Package shapes models simple 2-D shapes behind a small interface.
package shapes

import "errors"

// Shape is anything with an area and a perimeter.
type Shape interface {
	Area() float64
	Perimeter() float64
}

// TODO: add compile-time assertions proving that Circle, Rect and Triangle
// satisfy both Shape and fmt.Stringer, e.g.
//
//	var _ Shape = Circle{}

// ErrInvalidTriangle is returned (wrapped) by NewTriangle when the sides
// cannot form a triangle.
var ErrInvalidTriangle = errors.New("invalid triangle")

// Circle is a circle with the given radius.
type Circle struct {
	Radius float64
}

// Area returns πr².
func (c Circle) Area() float64 {
	return 0 // TODO
}

// Perimeter returns the circumference 2πr.
func (c Circle) Perimeter() float64 {
	return 0 // TODO
}

// String formats the circle as "Circle(r=2)". Numbers use the shortest
// representation that round-trips, as %g or strconv.FormatFloat(f, 'g', -1, 64) do.
func (c Circle) String() string {
	return "" // TODO
}

// Rect is an axis-aligned rectangle.
type Rect struct {
	Width, Height float64
}

// Area returns width × height.
func (r Rect) Area() float64 {
	return 0 // TODO
}

// Perimeter returns 2 × (width + height).
func (r Rect) Perimeter() float64 {
	return 0 // TODO
}

// String formats the rectangle as "Rect(3x4)".
func (r Rect) String() string {
	return "" // TODO
}

// Triangle is a triangle described by its three side lengths.
// Construct it with NewTriangle so the sides are validated.
type Triangle struct {
	A, B, C float64
}

// NewTriangle returns a Triangle with sides a, b and c, or an error wrapping
// ErrInvalidTriangle if any side is not positive or the triangle
// inequality does not hold strictly (degenerate triangles such as 1, 2, 3
// are rejected). On error it returns the zero Triangle.
func NewTriangle(a, b, c float64) (Triangle, error) {
	// TODO: validate.
	return Triangle{A: a, B: b, C: c}, nil
}

// Area uses Heron's formula: with s = perimeter/2,
// area = sqrt(s(s-a)(s-b)(s-c)).
func (t Triangle) Area() float64 {
	return 0 // TODO
}

// Perimeter returns the sum of the sides.
func (t Triangle) Perimeter() float64 {
	return 0 // TODO
}

// String formats the triangle as "Triangle(3, 4, 5)".
func (t Triangle) String() string {
	return "" // TODO
}

// TotalArea returns the sum of the areas of shapes. It returns 0 for an
// empty or nil slice.
func TotalArea(shapes []Shape) float64 {
	return 0 // TODO
}

// Largest returns the shape with the greatest area. If several shapes tie,
// the first one wins. ok is false when shapes is empty.
func Largest(shapes []Shape) (largest Shape, ok bool) {
	return nil, false // TODO
}

// SortByArea sorts shapes in place by ascending area. The sort must be
// stable: shapes with equal area keep their original relative order.
// Hint: slices.SortStableFunc and cmp.Compare.
func SortByArea(shapes []Shape) {
	// TODO
}
