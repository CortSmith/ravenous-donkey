package safediv

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
)

func TestDivide(t *testing.T) {
	tests := []struct {
		a, b int
		want int
	}{
		{10, 2, 5},
		{7, 2, 3},
		{-7, 2, -3}, // truncates toward zero
		{7, -2, -3},
		{0, 5, 0},
		{math.MaxInt, 1, math.MaxInt},
		{math.MinInt, 1, math.MinInt},
		{math.MinInt, 2, math.MinInt / 2},
		{math.MaxInt, -1, -math.MaxInt},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d/%d", tt.a, tt.b), func(t *testing.T) {
			got, err := Divide(tt.a, tt.b)
			if err != nil {
				t.Fatalf("Divide(%d, %d) unexpected error: %v", tt.a, tt.b, err)
			}
			if got != tt.want {
				t.Errorf("Divide(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestDivideErrors(t *testing.T) {
	tests := []struct {
		a, b    int
		wantIs  error
		wantMsg string
	}{
		{7, 0, ErrDivideByZero, "divide 7 by 0: division by zero"},
		{0, 0, ErrDivideByZero, "divide 0 by 0: division by zero"},
		{math.MinInt, -1, ErrOverflow, fmt.Sprintf("divide %d by -1: integer overflow", math.MinInt)},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d/%d", tt.a, tt.b), func(t *testing.T) {
			var got int
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Divide(%d, %d) panicked: %v; it must return an error instead", tt.a, tt.b, r)
					}
				}()
				got, err = Divide(tt.a, tt.b)
			}()
			if err == nil {
				t.Fatalf("Divide(%d, %d) = %d, nil; want error %v", tt.a, tt.b, got, tt.wantIs)
			}
			if got != 0 {
				t.Errorf("Divide(%d, %d) returned %d alongside an error; want 0", tt.a, tt.b, got)
			}
			if !errors.Is(err, tt.wantIs) {
				t.Errorf("errors.Is(err, %v) = false; err = %v", tt.wantIs, err)
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("Divide(%d, %d) error = %q, want %q", tt.a, tt.b, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestSafelyNoPanic(t *testing.T) {
	if err := Safely(func() error { return nil }); err != nil {
		t.Errorf("Safely(returns nil) = %v, want nil", err)
	}

	want := errors.New("ordinary failure")
	got := Safely(func() error { return want })
	if got != want {
		t.Errorf("Safely(returns err) = %v (%T), want the exact same error value", got, got)
	}
	var pe *PanicError
	if errors.As(got, &pe) {
		t.Errorf("Safely turned an ordinary error into a *PanicError")
	}
}

func TestSafelyRecovers(t *testing.T) {
	errSentinel := errors.New("custom panic error")
	zero := 0
	var nilMap map[string]int
	var nilPtr *struct{ X int }

	tests := []struct {
		name         string
		f            func() error
		wantMsgHas   string // substring of err.Error()
		wantRuntime  bool   // cause is a runtime.Error
		wantIs       error  // optional errors.Is target
		wantUnwrapNl bool   // Unwrap() must be nil (non-error panic value)
	}{
		{
			name:         "string value",
			f:            func() error { panic("something broke") },
			wantMsgHas:   "panic: something broke",
			wantUnwrapNl: true,
		},
		{
			name:         "int value",
			f:            func() error { panic(42) },
			wantMsgHas:   "panic: 42",
			wantUnwrapNl: true,
		},
		{
			name:       "error value is unwrappable",
			f:          func() error { panic(fmt.Errorf("wrapped: %w", errSentinel)) },
			wantMsgHas: "panic: wrapped: custom panic error",
			wantIs:     errSentinel,
		},
		{
			name: "integer divide by zero",
			f: func() error {
				_ = 1 / zero
				return nil
			},
			wantMsgHas:  "integer divide by zero",
			wantRuntime: true,
		},
		{
			name: "index out of range",
			f: func() error {
				s := []int{1, 2, 3}
				i := 5
				_ = s[i]
				return nil
			},
			wantMsgHas:  "index out of range",
			wantRuntime: true,
		},
		{
			name: "nil map write",
			f: func() error {
				nilMap["x"] = 1
				return nil
			},
			wantMsgHas:  "nil map",
			wantRuntime: true,
		},
		{
			name: "nil pointer dereference",
			f: func() error {
				return fmt.Errorf("%d", nilPtr.X)
			},
			wantMsgHas:  "nil pointer dereference",
			wantRuntime: true,
		},
		{
			// Since Go 1.21, panic(nil) becomes *runtime.PanicNilError so
			// recover() never returns nil for a real panic.
			name:        "panic(nil)",
			f:           func() error { panic(nil) },
			wantMsgHas:  "panic called with nil argument",
			wantRuntime: true,
		},
		{
			name:        "nil function",
			f:           nil,
			wantMsgHas:  "nil pointer dereference",
			wantRuntime: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic escaped Safely: %v", r)
					}
				}()
				err = Safely(tt.f)
			}()

			var pe *PanicError
			if !errors.As(err, &pe) {
				t.Fatalf("Safely(...) = %v (%T), want *PanicError", err, err)
			}
			if !strings.Contains(err.Error(), tt.wantMsgHas) {
				t.Errorf("err.Error() = %q, want it to contain %q", err.Error(), tt.wantMsgHas)
			}
			if !strings.HasPrefix(err.Error(), "panic: ") {
				t.Errorf("err.Error() = %q, want prefix %q", err.Error(), "panic: ")
			}
			if len(pe.Stack) == 0 {
				t.Errorf("PanicError.Stack is empty; capture it with runtime/debug.Stack()")
			} else if !strings.Contains(string(pe.Stack), "safediv") {
				t.Errorf("PanicError.Stack does not mention this package:\n%s", pe.Stack)
			}
			if tt.wantRuntime {
				var re runtime.Error
				if !errors.As(err, &re) {
					t.Errorf("errors.As(err, runtime.Error) = false; PanicError.Unwrap should expose error values (Value = %#v)", pe.Value)
				}
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Errorf("errors.Is(err, %v) = false; err = %v", tt.wantIs, err)
			}
			if tt.wantUnwrapNl && pe.Unwrap() != nil {
				t.Errorf("PanicError.Unwrap() = %v, want nil for non-error value %#v", pe.Unwrap(), pe.Value)
			}
		})
	}
}

// Safely must be usable around code that itself uses Safely.
func TestSafelyNested(t *testing.T) {
	var inner error
	outer := Safely(func() error {
		inner = Safely(func() error { panic("inner") })
		return fmt.Errorf("outer saw: %w", inner)
	})
	var pe *PanicError
	if !errors.As(inner, &pe) || pe.Value != "inner" {
		t.Fatalf("inner = %v, want *PanicError with Value \"inner\"", inner)
	}
	if !errors.As(outer, &pe) {
		t.Errorf("outer = %v; want it to wrap the inner *PanicError", outer)
	}
	if got, want := outer.Error(), "outer saw: panic: inner"; got != want {
		t.Errorf("outer.Error() = %q, want %q", got, want)
	}
}

// Divide plus Safely: the error path and the panic path look different.
func TestDivideVersusRawPanic(t *testing.T) {
	zero := 0
	raw := Safely(func() error { _ = 10 / zero; return nil })
	_, safe := Divide(10, zero)

	var pe *PanicError
	if !errors.As(raw, &pe) {
		t.Errorf("raw division by zero should panic and be recovered as *PanicError, got %v", raw)
	}
	if errors.As(safe, &pe) {
		t.Errorf("Divide should return a plain error, not *PanicError: %v", safe)
	}
	if !errors.Is(safe, ErrDivideByZero) {
		t.Errorf("errors.Is(Divide(10, 0), ErrDivideByZero) = false: %v", safe)
	}
}
