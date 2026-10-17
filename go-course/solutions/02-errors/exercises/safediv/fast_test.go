package safediv

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestSafelyFast(t *testing.T) {
	if err := SafelyFast(func() error { return nil }); err != nil {
		t.Errorf("SafelyFast(nil-returning) = %v", err)
	}
	zero := 0
	err := SafelyFast(func() error { _ = 1 / zero; return nil })
	var le *LazyPanicError
	if !errors.As(err, &le) {
		t.Fatalf("SafelyFast(div by zero) = %v (%T), want *LazyPanicError", err, err)
	}
	var re runtime.Error
	if !errors.As(err, &re) {
		t.Errorf("errors.As(err, runtime.Error) = false")
	}
	if st := le.Stack(); !strings.Contains(st, "safediv") {
		t.Errorf("Stack() does not mention the package:\n%s", st)
	}
}

var sinkErr error

func BenchmarkSafelyNoPanic(b *testing.B) {
	f := func() error { return nil }
	b.ReportAllocs()
	for b.Loop() {
		sinkErr = Safely(f)
	}
}

func BenchmarkSafelyPanic(b *testing.B) {
	f := func() error { panic("boom") }
	b.ReportAllocs()
	for b.Loop() {
		sinkErr = Safely(f)
	}
}

func BenchmarkSafelyFastPanic(b *testing.B) {
	f := func() error { panic("boom") }
	b.ReportAllocs()
	for b.Loop() {
		sinkErr = SafelyFast(f)
	}
}
