package describe

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

type celsius float64

func (c celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

// both implements error AND fmt.Stringer; error must win.
type both struct{}

func (both) Error() string  { return "boom" }
func (both) String() string { return "pretty" }

type point struct{ X, Y int }

func TestDescribe(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, "nil"},
		{"bool true", true, "bool true"},
		{"bool false", false, "bool false"},
		{"int", 42, "int 42"},
		{"negative int8", int8(-3), "int8 -3"},
		{"int16", int16(300), "int16 300"},
		{"rune is int32", 'a', "int32 97"},
		{"int64", int64(-1 << 40), "int64 -1099511627776"},
		{"uint", uint(7), "uint 7"},
		{"byte is uint8", byte(255), "uint8 255"},
		{"uint16", uint16(65535), "uint16 65535"},
		{"uint32", uint32(1), "uint32 1"},
		{"uint64 max", uint64(18446744073709551615), "uint64 18446744073709551615"},
		{"float64", 2.5, "float64 2.5"},
		{"float64 integral", 3.0, "float64 3"},
		{"float64 tiny", 1e-9, "float64 1e-09"},
		{"float32 shortest", float32(0.1), "float32 0.1"},
		{"string", "hi", `string "hi"`},
		{"string needing quotes", "a\"b\n", `string "a\"b\n"`},
		{"empty string", "", `string ""`},
		{"bytes", []byte("abc"), "[]byte len=3"},
		{"nil bytes", []byte(nil), "[]byte len=0"},
		{"error", errors.New("disk full"), "error: disk full"},
		{"wrapped error", fmt.Errorf("save: %w", errors.New("disk full")), "error: save: disk full"},
		{"stringer", celsius(21.5), "Stringer: 21.5°C"},
		{"stdlib stringer", time.Duration(1500) * time.Millisecond, "Stringer: 1.5s"},
		{"error beats stringer", both{}, "error: boom"},
		{"empty []any", []any{}, "[]"},
		{"[]any recursive", []any{1, "x", nil, []any{true}}, `[int 1, string "x", nil, [bool true]]`},
		{"map sorted", map[string]any{"b": 2, "a": "one", "c": nil}, `map[a: string "one", b: int 2, c: nil]`},
		{"empty map", map[string]any{}, "map[]"},
		{"nested map in slice", []any{map[string]any{"k": 1.5}}, "[map[k: float64 1.5]]"},
		{"unknown slice", []int{1, 2}, "unknown []int"},
		{"unknown struct", point{1, 2}, "unknown describe.point"},
		{"unknown pointer", &point{}, "unknown *describe.point"},
		{"unknown map", map[int]int{}, "unknown map[int]int"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Describe(tt.in); got != tt.want {
				t.Errorf("Describe(%#v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateReturnsUntypedNil(t *testing.T) {
	for _, n := range []int{0, 1, 50, 100} {
		err := Validate(n)
		if err != nil {
			// %#v reveals the hidden dynamic type of the interface value.
			t.Errorf("Validate(%d) = %#v (dynamic type %T), want nil; "+
				"an interface holding a nil pointer is NOT a nil interface", n, err, err)
		}
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		n          int
		wantReason string
		wantMsg    string
	}{
		{-1, "negative", "invalid value -1: negative"},
		{-1000, "negative", "invalid value -1000: negative"},
		{101, "greater than 100", "invalid value 101: greater than 100"},
	}
	for _, tt := range tests {
		err := Validate(tt.n)
		if err == nil {
			t.Errorf("Validate(%d) = nil, want error", tt.n)
			continue
		}
		var verr *ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("Validate(%d) error type = %T, want *ValidationError", tt.n, err)
			continue
		}
		if verr.Value != tt.n || verr.Reason != tt.wantReason {
			t.Errorf("Validate(%d) = %+v, want Value=%d Reason=%q", tt.n, *verr, tt.n, tt.wantReason)
		}
		if got := err.Error(); got != tt.wantMsg {
			t.Errorf("Validate(%d).Error() = %q, want %q", tt.n, got, tt.wantMsg)
		}
	}
}
