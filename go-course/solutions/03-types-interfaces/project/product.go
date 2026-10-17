// Package inventory manages product stock: a Product value type, a Store
// interface with an in-memory implementation, and an Inventory service that
// sells, restocks, filters, sorts and reports.
package inventory

import (
	"errors"
	"fmt"
	"strconv"
)

// Sentinel errors. Callers test for them with errors.Is; functions wrap them
// with context using %w.
var (
	ErrNotFound          = errors.New("product not found")
	ErrDuplicate         = errors.New("product already exists")
	ErrInvalid           = errors.New("invalid product")
	ErrInsufficientStock = errors.New("insufficient stock")
)

// Product is a stock-keeping unit. It is a small value type: methods use
// value receivers and Products are copied freely in and out of a Store.
type Product struct {
	SKU          string
	Name         string
	Category     string
	PriceCents   int64 // money as integer cents: never use float64 for money
	Quantity     int
	ReorderLevel int // the product is "low stock" when Quantity <= ReorderLevel
}

// Validate reports the first problem with p, wrapping ErrInvalid. Checks,
// in order: SKU non-empty, Name non-empty, PriceCents >= 0, Quantity >= 0,
// ReorderLevel >= 0.
func (p Product) Validate() error {
	switch {
	case p.SKU == "":
		return fmt.Errorf("SKU is empty: %w", ErrInvalid)
	case p.Name == "":
		return fmt.Errorf("product %s: name is empty: %w", p.SKU, ErrInvalid)
	case p.PriceCents < 0:
		return fmt.Errorf("product %s: negative price %d: %w", p.SKU, p.PriceCents, ErrInvalid)
	case p.Quantity < 0:
		return fmt.Errorf("product %s: negative quantity %d: %w", p.SKU, p.Quantity, ErrInvalid)
	case p.ReorderLevel < 0:
		return fmt.Errorf("product %s: negative reorder level %d: %w", p.SKU, p.ReorderLevel, ErrInvalid)
	}
	return nil
}

// Value returns the stock value in cents (PriceCents × Quantity).
func (p Product) Value() int64 { return p.PriceCents * int64(p.Quantity) }

// LowStock reports whether Quantity <= ReorderLevel.
func (p Product) LowStock() bool { return p.Quantity <= p.ReorderLevel }

// String formats the product as `SKU "Name" qty=N @ $X.YY`.
func (p Product) String() string {
	return fmt.Sprintf("%s %q qty=%d @ %s", p.SKU, p.Name, p.Quantity, FormatCents(p.PriceCents))
}

// FormatCents formats an amount of cents as dollars: 199 -> "$1.99",
// 5 -> "$0.05", -250 -> "-$2.50", 123456 -> "$1234.56".
func FormatCents(cents int64) string {
	sign := ""
	// Work in uint64 so math.MinInt64 does not overflow when negated.
	u := uint64(cents)
	if cents < 0 {
		sign = "-"
		u = -u
	}
	frac := u % 100
	fracStr := strconv.FormatUint(frac, 10)
	if frac < 10 {
		fracStr = "0" + fracStr
	}
	return sign + "$" + strconv.FormatUint(u/100, 10) + "." + fracStr
}
