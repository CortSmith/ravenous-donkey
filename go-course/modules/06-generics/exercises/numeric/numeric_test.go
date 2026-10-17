package numeric

import (
	"math"
	"testing"
	"time"
)

// Named types whose underlying types are numeric. They only satisfy the
// constraints because of the ~ in Integer and Float.
type (
	Cents   int64
	Celsius float64
	Port    uint16
)

func TestSumInts(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want int
	}{
		{"empty", nil, 0},
		{"one", []int{7}, 7},
		{"several", []int{1, 2, 3, 4}, 10},
		{"negatives", []int{-5, 3, -1}, -3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Sum(tt.in...); got != tt.want {
				t.Errorf("Sum(%v...) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestSumOtherTypes(t *testing.T) {
	if got := Sum(1.5, 2.25); got != 3.75 {
		t.Errorf("Sum(1.5, 2.25) = %v, want 3.75", got)
	}
	if got := Sum[float32](0.5, 0.25); got != 0.75 {
		t.Errorf("Sum[float32](0.5, 0.25) = %v, want 0.75", got)
	}
	if got := Sum(Cents(199), Cents(1)); got != Cents(200) {
		t.Errorf("Sum(Cents(199), Cents(1)) = %v, want 200", got)
	}
	if got := Sum[Celsius](20, 1.5); got != 21.5 {
		t.Errorf("Sum[Celsius](20, 1.5) = %v, want 21.5", got)
	}
	// time.Duration is `type Duration int64`, so it is an Integer too.
	if got := Sum(time.Second, 500*time.Millisecond); got != 1500*time.Millisecond {
		t.Errorf("Sum(1s, 500ms) = %v, want 1.5s", got)
	}
}

func TestSumKeepsElementTypeAndWraps(t *testing.T) {
	// Sum[uint8] adds in uint8, so 200+100 wraps to 44, just as
	// uint8(200)+uint8(100) does. This documents the overflow behaviour.
	if got := Sum[uint8](200, 100); got != 44 {
		t.Errorf("Sum[uint8](200, 100) = %d, want 44 (wrapping uint8 arithmetic)", got)
	}
	if got := Sum[Port](65535, 2); got != 1 {
		t.Errorf("Sum[Port](65535, 2) = %d, want 1 (wrapping uint16 arithmetic)", got)
	}
}

func TestSumBy(t *testing.T) {
	type item struct {
		Name  string
		Price Cents
		Qty   int
	}
	cart := []item{{"tea", 450, 2}, {"cake", 325, 1}, {"jam", 0, 3}}

	if got := SumBy(cart, func(i item) Cents { return i.Price * Cents(i.Qty) }); got != 1225 {
		t.Errorf("SumBy(cart, price*qty) = %d, want 1225", got)
	}
	if got := SumBy(cart, func(i item) int { return i.Qty }); got != 6 {
		t.Errorf("SumBy(cart, qty) = %d, want 6", got)
	}
	if got := SumBy([]string{"a", "bcd"}, func(s string) float64 { return float64(len(s)) / 2 }); got != 2 {
		t.Errorf("SumBy([a bcd], len/2) = %v, want 2", got)
	}
	if got := SumBy([]item(nil), func(i item) int { return i.Qty }); got != 0 {
		t.Errorf("SumBy(nil, qty) = %d, want 0", got)
	}
}

func TestMean(t *testing.T) {
	// Each case wraps the call in a closure because Mean is instantiated with
	// a different element type in each row.
	tests := []struct {
		name   string
		call   func() (float64, bool)
		want   float64
		wantOK bool
	}{
		{"ints", func() (float64, bool) { return Mean([]int{1, 2, 3, 4}) }, 2.5, true},
		{"single", func() (float64, bool) { return Mean([]float64{-3}) }, -3, true},
		{"named float", func() (float64, bool) { return Mean([]Celsius{18, 22}) }, 20, true},
		{"empty", func() (float64, bool) { return Mean([]int{}) }, 0, false},
		{"nil", func() (float64, bool) { return Mean[uint](nil) }, 0, false},
		// Summing these as int8 would overflow (100+100 = -56). Converting to
		// float64 first gives the right answer.
		{"no overflow in small ints", func() (float64, bool) { return Mean([]int8{100, 100, 100}) }, 100, true},
		{"no overflow in int64", func() (float64, bool) { return Mean([]int64{math.MaxInt64, math.MaxInt64}) }, math.MaxInt64, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.call()
			if ok != tt.wantOK || math.Abs(got-tt.want) > 1e-9*math.Max(1, math.Abs(tt.want)) {
				t.Errorf("Mean = (%v, %v), want (%v, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	intTests := []struct{ v, lo, hi, want int }{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{11, 0, 10, 10},
		{0, 0, 10, 0},
		{10, 0, 10, 10},
		{3, 3, 3, 3},
	}
	for _, tt := range intTests {
		if got := Clamp(tt.v, tt.lo, tt.hi); got != tt.want {
			t.Errorf("Clamp(%d, %d, %d) = %d, want %d", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
	if got := Clamp("m", "a", "k"); got != "k" {
		t.Errorf(`Clamp("m", "a", "k") = %q, want "k" (strings are cmp.Ordered too)`, got)
	}
	if got := Clamp(Celsius(-40), -10, 50); got != -10 {
		t.Errorf("Clamp(Celsius(-40), -10, 50) = %v, want -10", got)
	}
}

func TestClampPanicsOnInvertedRange(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Clamp(5, 10, 0) did not panic, want a panic because lo > hi")
		}
	}()
	Clamp(5, 10, 0)
}
