package shapes

import (
	"errors"
	"fmt"
	"math"
	"testing"
)

// These lines fail to COMPILE if a type stops satisfying the interfaces,
// e.g. because a method was given a pointer receiver.
var (
	_ Shape        = Circle{}
	_ Shape        = Rect{}
	_ Shape        = Triangle{}
	_ fmt.Stringer = Circle{}
	_ fmt.Stringer = Rect{}
	_ fmt.Stringer = Triangle{}
)

const eps = 1e-9

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) <= eps*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

func mustTriangle(t *testing.T, a, b, c float64) Triangle {
	t.Helper()
	tri, err := NewTriangle(a, b, c)
	if err != nil {
		t.Fatalf("NewTriangle(%g, %g, %g) returned unexpected error: %v", a, b, c, err)
	}
	return tri
}

func TestAreaAndPerimeter(t *testing.T) {
	tests := []struct {
		name          string
		shape         Shape
		wantArea      float64
		wantPerimeter float64
	}{
		{"unit circle", Circle{Radius: 1}, math.Pi, 2 * math.Pi},
		{"circle r=2.5", Circle{Radius: 2.5}, math.Pi * 6.25, 5 * math.Pi},
		{"zero circle", Circle{}, 0, 0},
		{"rect 3x4", Rect{Width: 3, Height: 4}, 12, 14},
		{"square", Rect{Width: 2.5, Height: 2.5}, 6.25, 10},
		{"zero rect", Rect{}, 0, 0},
		{"3-4-5 triangle", Triangle{A: 3, B: 4, C: 5}, 6, 12},
		{"equilateral", Triangle{A: 2, B: 2, C: 2}, math.Sqrt(3), 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.shape.Area(); !approxEqual(got, tt.wantArea) {
				t.Errorf("%#v.Area() = %v, want %v", tt.shape, got, tt.wantArea)
			}
			if got := tt.shape.Perimeter(); !approxEqual(got, tt.wantPerimeter) {
				t.Errorf("%#v.Perimeter() = %v, want %v", tt.shape, got, tt.wantPerimeter)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		shape fmt.Stringer
		want  string
	}{
		{Circle{Radius: 2}, "Circle(r=2)"},
		{Circle{Radius: 1.5}, "Circle(r=1.5)"},
		{Rect{Width: 3, Height: 4}, "Rect(3x4)"},
		{Rect{Width: 0.25, Height: 10}, "Rect(0.25x10)"},
		{Triangle{A: 3, B: 4, C: 5}, "Triangle(3, 4, 5)"},
		{Triangle{A: 1.5, B: 2, C: 2.5}, "Triangle(1.5, 2, 2.5)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.shape.String(); got != tt.want {
				t.Errorf("%#v.String() = %q, want %q", tt.shape, got, tt.want)
			}
			// fmt must pick up the Stringer for both values and pointers.
			if got := fmt.Sprint(tt.shape); got != tt.want {
				t.Errorf("fmt.Sprint(%#v) = %q, want %q (is String defined with a value receiver?)", tt.shape, got, tt.want)
			}
		})
	}
	// A pointer to a value-receiver Stringer is also a Stringer.
	if got, want := fmt.Sprintf("%v", &Rect{Width: 1, Height: 2}), "Rect(1x2)"; got != want {
		t.Errorf("fmt.Sprintf(%%v, &Rect{1, 2}) = %q, want %q", got, want)
	}
}

func TestNewTriangle(t *testing.T) {
	tests := []struct {
		name    string
		a, b, c float64
		wantErr bool
	}{
		{"right triangle", 3, 4, 5, false},
		{"equilateral", 1, 1, 1, false},
		{"order does not matter", 5, 3, 4, false},
		{"degenerate (a+b == c)", 1, 2, 3, true},
		{"impossible", 1, 1, 10, true},
		{"impossible, long side first", 10, 1, 1, true},
		{"zero side", 0, 4, 5, true},
		{"negative side", -3, 4, 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tri, err := NewTriangle(tt.a, tt.b, tt.c)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewTriangle(%g, %g, %g) = %v, nil; want error", tt.a, tt.b, tt.c, tri)
				}
				if !errors.Is(err, ErrInvalidTriangle) {
					t.Errorf("NewTriangle(%g, %g, %g) error = %v; want it to wrap ErrInvalidTriangle", tt.a, tt.b, tt.c, err)
				}
				if tri != (Triangle{}) {
					t.Errorf("NewTriangle(%g, %g, %g) returned %#v with an error; want the zero Triangle", tt.a, tt.b, tt.c, tri)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTriangle(%g, %g, %g) error = %v, want nil", tt.a, tt.b, tt.c, err)
			}
			want := Triangle{A: tt.a, B: tt.b, C: tt.c}
			if tri != want {
				t.Errorf("NewTriangle(%g, %g, %g) = %#v, want %#v", tt.a, tt.b, tt.c, tri, want)
			}
		})
	}
}

func TestTotalArea(t *testing.T) {
	tests := []struct {
		name   string
		shapes []Shape
		want   float64
	}{
		{"nil slice", nil, 0},
		{"empty slice", []Shape{}, 0},
		{"single", []Shape{Rect{Width: 2, Height: 3}}, 6},
		{"mixed", []Shape{Rect{Width: 2, Height: 3}, Triangle{A: 3, B: 4, C: 5}, Circle{Radius: 1}}, 12 + math.Pi},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TotalArea(tt.shapes); !approxEqual(got, tt.want) {
				t.Errorf("TotalArea(%v) = %v, want %v", tt.shapes, got, tt.want)
			}
		})
	}
}

func TestLargest(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got, ok := Largest(nil)
		if ok || got != nil {
			t.Errorf("Largest(nil) = %v, %v; want nil, false", got, ok)
		}
	})
	t.Run("mixed", func(t *testing.T) {
		shapes := []Shape{Rect{Width: 1, Height: 1}, Circle{Radius: 2}, mustTriangle(t, 3, 4, 5)}
		got, ok := Largest(shapes)
		if !ok || got != (Circle{Radius: 2}) {
			t.Errorf("Largest(%v) = %v, %v; want Circle(r=2), true", shapes, got, ok)
		}
	})
	t.Run("tie keeps first", func(t *testing.T) {
		shapes := []Shape{Rect{Width: 1, Height: 1}, Rect{Width: 2, Height: 3}, Rect{Width: 3, Height: 2}}
		got, ok := Largest(shapes)
		if !ok || got != (Rect{Width: 2, Height: 3}) {
			t.Errorf("Largest(%v) = %v, %v; want Rect(2x3), true", shapes, got, ok)
		}
	})
}

func TestSortByArea(t *testing.T) {
	shapes := []Shape{
		Circle{Radius: 2},           // 12.566
		Rect{Width: 3, Height: 2},   // 6  (A)
		Triangle{A: 3, B: 4, C: 5},  // 6  (B) – must stay after A
		Rect{Width: 1, Height: 1},   // 1
		Rect{Width: 2, Height: 3},   // 6  (C) – must stay after B
		Circle{Radius: 0.5},         // 0.785
		Rect{Width: 100, Height: 0}, // 0
	}
	want := []Shape{
		Rect{Width: 100, Height: 0},
		Circle{Radius: 0.5},
		Rect{Width: 1, Height: 1},
		Rect{Width: 3, Height: 2},
		Triangle{A: 3, B: 4, C: 5},
		Rect{Width: 2, Height: 3},
		Circle{Radius: 2},
	}
	SortByArea(shapes)
	for i := range want {
		if shapes[i] != want[i] {
			t.Fatalf("SortByArea result = %v\nwant              %v\n(first difference at index %d; is the sort stable?)", shapes, want, i)
		}
	}

	// Sorting nil or a single element must not panic.
	SortByArea(nil)
	one := []Shape{Circle{Radius: 1}}
	SortByArea(one)
	if one[0] != (Circle{Radius: 1}) {
		t.Errorf("SortByArea on a single element changed it to %v", one[0])
	}
}
