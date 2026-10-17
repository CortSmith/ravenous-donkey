// Package numeric provides generic arithmetic helpers built on type-set
// constraints.
package numeric

import "cmp"

// Integer is satisfied by every integer type and by any named type whose
// underlying type is an integer type (the ~ makes that work).
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Float is satisfied by float32, float64 and named types based on them.
type Float interface {
	~float32 | ~float64
}

// Number is the union of Integer and Float. It is a constraint only: it
// cannot be used as the type of a variable.
type Number interface {
	Integer | Float
}

// Sum returns the sum of xs, or 0 if xs is empty. The result has the same
// type as the elements, so integer sums wrap on overflow exactly as + does.
func Sum[T Number](xs ...T) T {
	var total T
	for _, x := range xs {
		total += x
	}
	return total
}

// SumBy returns the sum of f(e) for every element e of xs.
func SumBy[E any, N Number](xs []E, f func(E) N) N {
	var total N
	for _, e := range xs {
		total += f(e)
	}
	return total
}

// Mean returns the arithmetic mean of xs as a float64, and false if xs is
// empty. Each element is converted to float64 before summing, so integer
// inputs cannot overflow (though very large int64 values lose precision).
func Mean[T Number](xs []T) (float64, bool) {
	if len(xs) == 0 {
		return 0, false
	}
	var total float64
	for _, x := range xs {
		total += float64(x)
	}
	return total / float64(len(xs)), true
}

// Clamp returns v limited to the closed range [lo, hi]. It panics if lo > hi,
// because that is a programming error in the caller.
func Clamp[T cmp.Ordered](v, lo, hi T) T {
	if cmp.Less(hi, lo) {
		panic("numeric.Clamp: lo > hi")
	}
	return min(max(v, lo), hi)
}
