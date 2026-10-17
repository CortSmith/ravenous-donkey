// Package inventory manages product stock: a Product value type, a Store
// interface with an in-memory implementation, and an Inventory service that
// sells, restocks, filters, sorts and reports.
package inventory

import "errors"

// Sentinel errors. Callers test for them with errors.Is; functions wrap them
// with context using %w.
var (
	ErrNotFound          = errors.New("product not found")
	ErrDuplicate         = errors.New("product already exists")
	ErrInvalid           = errors.New("invalid product")
	ErrInsufficientStock = errors.New("insufficient stock")
)

// Product is a stock-keeping unit. It is a small value type.
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
// ReorderLevel >= 0. The message must mention the offending field
// ("SKU", "name", "price", "quantity", "reorder").
func (p Product) Validate() error {
	return nil // TODO
}

// Value returns the stock value in cents (PriceCents × Quantity).
func (p Product) Value() int64 {
	return 0 // TODO
}

// LowStock reports whether Quantity <= ReorderLevel.
func (p Product) LowStock() bool {
	return false // TODO
}

// String formats the product as `SKU "Name" qty=N @ $X.YY`, e.g.
// `A-1 "Widget" qty=3 @ $1.99`.
func (p Product) String() string {
	return "" // TODO
}

// FormatCents formats an amount of cents as dollars: 199 -> "$1.99",
// 5 -> "$0.05", -250 -> "-$2.50", 123456 -> "$1234.56". It must handle the
// full int64 range, including math.MinInt64.
func FormatCents(cents int64) string {
	return "" // TODO
}
