package ledger

import (
	"errors"
	"math"
	"testing"
)

func TestParseAmount(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"0.00", 0},
		{"1", 100},
		{"12", 1200},
		{"12.5", 1250},
		{"12.50", 1250},
		{"12.05", 1205},
		{"0.01", 1},
		{"007.10", 710},
		{"92233720368547758.07", math.MaxInt64},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseAmount(tt.in)
			if err != nil {
				t.Fatalf("ParseAmount(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseAmount(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseAmountInvalid(t *testing.T) {
	inputs := []string{
		"", " ", "abc", "-5", "+5", "1.234", "1.", ".5", "1..2", "1.2.3",
		"1e3", "1,000", " 1", "1 ", "0x10", "١٢", // Arabic-Indic digits are not ASCII
		"92233720368547758.08", // MaxInt64 + 1 cent
		"99999999999999999999", // overflows ParseInt itself
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			got, err := ParseAmount(in)
			if err == nil {
				t.Fatalf("ParseAmount(%q) = %d, nil; want error", in, got)
			}
			if !errors.Is(err, ErrInvalidAmount) {
				t.Errorf("errors.Is(ParseAmount(%q), ErrInvalidAmount) = false; err = %v", in, err)
			}
			if got != 0 {
				t.Errorf("ParseAmount(%q) returned %d alongside an error; want 0", in, got)
			}
		})
	}
}

func TestParseAmountMessage(t *testing.T) {
	_, err := ParseAmount("1.234")
	if want := `invalid amount "1.234"`; err == nil || err.Error() != want {
		t.Errorf("ParseAmount(%q) error = %v, want %q", "1.234", err, want)
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0.00"},
		{5, "0.05"},
		{1234, "12.34"},
		{100, "1.00"},
		{-150, "-1.50"},
		{math.MaxInt64, "92233720368547758.07"},
		{math.MinInt64, "-92233720368547758.08"},
	}
	for _, tt := range tests {
		if got := FormatAmount(tt.in); got != tt.want {
			t.Errorf("FormatAmount(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
