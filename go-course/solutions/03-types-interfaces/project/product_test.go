package inventory

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
)

var _ fmt.Stringer = Product{}

func TestFormatCents(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "$0.00"},
		{5, "$0.05"},
		{10, "$0.10"},
		{99, "$0.99"},
		{100, "$1.00"},
		{199, "$1.99"},
		{123456, "$1234.56"},
		{-5, "-$0.05"},
		{-250, "-$2.50"},
		{math.MaxInt64, "$92233720368547758.07"},
		{math.MinInt64, "-$92233720368547758.08"},
	}
	for _, tt := range tests {
		if got := FormatCents(tt.in); got != tt.want {
			t.Errorf("FormatCents(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestProductString(t *testing.T) {
	p := Product{SKU: "A-1", Name: "Widget", PriceCents: 199, Quantity: 3}
	want := `A-1 "Widget" qty=3 @ $1.99`
	if got := p.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := fmt.Sprint(p); got != want {
		t.Errorf("fmt.Sprint(p) = %q, want %q (use a value receiver)", got, want)
	}
}

func TestValueAndLowStock(t *testing.T) {
	tests := []struct {
		p         Product
		wantValue int64
		wantLow   bool
	}{
		{Product{PriceCents: 199, Quantity: 10, ReorderLevel: 2}, 1990, false},
		{Product{PriceCents: 199, Quantity: 2, ReorderLevel: 2}, 398, true}, // equal counts as low
		{Product{PriceCents: 500, Quantity: 0, ReorderLevel: 0}, 0, true},
		{Product{PriceCents: 0, Quantity: 7, ReorderLevel: 1}, 0, false},
		{Product{PriceCents: 300_000_000, Quantity: 1_000_000}, 300_000_000_000_000, false}, // no int overflow
	}
	for _, tt := range tests {
		if got := tt.p.Value(); got != tt.wantValue {
			t.Errorf("%+v.Value() = %d, want %d", tt.p, got, tt.wantValue)
		}
		if got := tt.p.LowStock(); got != tt.wantLow {
			t.Errorf("%+v.LowStock() = %v, want %v", tt.p, got, tt.wantLow)
		}
	}
}

func TestValidate(t *testing.T) {
	valid := Product{SKU: "A-1", Name: "Widget", Category: "tools", PriceCents: 199, Quantity: 3, ReorderLevel: 1}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() on valid product = %v, want nil", err)
	}
	if err := (Product{SKU: "Z", Name: "Freebie"}).Validate(); err != nil {
		t.Errorf("Validate() on zero price/qty/reorder = %v, want nil (zero is allowed)", err)
	}

	tests := []struct {
		name    string
		mutate  func(*Product)
		wantMsg string
	}{
		{"empty SKU", func(p *Product) { p.SKU = "" }, "SKU"},
		{"empty name", func(p *Product) { p.Name = "" }, "name"},
		{"negative price", func(p *Product) { p.PriceCents = -1 }, "price"},
		{"negative quantity", func(p *Product) { p.Quantity = -1 }, "quantity"},
		{"negative reorder level", func(p *Product) { p.ReorderLevel = -1 }, "reorder"},
		// Several problems: the first check in the documented order wins.
		{"name checked before price", func(p *Product) { p.Name = ""; p.PriceCents = -1 }, "name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := valid // Product is a value: this is a copy
			tt.mutate(&p)
			err := p.Validate()
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Validate(%+v) = %v, want error wrapping ErrInvalid", p, err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Validate(%+v) = %q, want message mentioning %q", p, err, tt.wantMsg)
			}
		})
	}
}
