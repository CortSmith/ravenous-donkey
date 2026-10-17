package inventory

import (
	"errors"
	"io"
)

// Alerter is notified when a sale pushes a product into low stock.
type Alerter interface {
	LowStock(p Product)
}

// AlerterFunc adapts an ordinary function to the Alerter interface, in the
// same way http.HandlerFunc adapts functions to http.Handler.
type AlerterFunc func(Product)

// LowStock calls f(p).
func (f AlerterFunc) LowStock(p Product) {
	// TODO
}

// Inventory is the business layer on top of a Store.
//
// TODO: Sell and Receive do Get-then-Update. The Store's own locking only
// protects each call individually, so concurrent sales can lose updates.
// Add what you need to make the whole sequence atomic.
type Inventory struct {
	store   Store
	alerter Alerter
}

// New returns an Inventory backed by store. alerter may be nil.
func New(store Store, alerter Alerter) *Inventory {
	return &Inventory{store: store, alerter: alerter}
}

// AddProduct validates p and adds it to the store.
func (inv *Inventory) AddProduct(p Product) error {
	return errors.New("not implemented") // TODO
}

// Receive adds qty units of stock to the product. qty must be positive
// (else an error wrapping ErrInvalid). It never triggers an alert.
func (inv *Inventory) Receive(sku string, qty int) error {
	return errors.New("not implemented") // TODO
}

// Sell removes qty units of stock. qty must be positive (else ErrInvalid);
// selling more than is in stock fails with an error wrapping
// ErrInsufficientStock (mentioning the SKU) and changes nothing. If the sale
// moves the product from not-low to low stock, the alerter (if any) is
// called once with the updated product.
func (inv *Inventory) Sell(sku string, qty int) error {
	return errors.New("not implemented") // TODO
}

// Filter returns the products for which keep returns true, sorted by SKU.
// It returns an empty (non-nil) slice when nothing matches.
func (inv *Inventory) Filter(keep func(Product) bool) []Product {
	return nil // TODO
}

// LowStock returns the low-stock products sorted by SKU.
func (inv *Inventory) LowStock() []Product {
	return nil // TODO (one line, using Filter)
}

// TotalValue returns the sum of Value() over all products, in cents.
func (inv *Inventory) TotalValue() int64 {
	return 0 // TODO
}

// SortKey selects the order used by Sorted and WriteReport.
type SortKey int

// Sort orders. Every order breaks ties by ascending SKU so output is
// deterministic.
const (
	BySKU      SortKey = iota // ascending SKU
	ByName                    // ascending Name
	ByValue                   // descending Value()
	ByQuantity                // ascending Quantity
)

// String returns the lower-case name of the key ("sku", "name", "value",
// "quantity") or "SortKey(N)" for unknown values.
func (k SortKey) String() string {
	return "" // TODO
}

// Sorted returns all products in the order given by key. Unknown keys
// return an error wrapping ErrInvalid.
func (inv *Inventory) Sorted(key SortKey) ([]Product, error) {
	return nil, errors.New("not implemented") // TODO
}

// WriteReport writes a fixed-width stock report to w, ordered by key:
//
//	SKU      NAME                   QTY      PRICE        VALUE
//	A-1      Widget                  10      $1.99       $19.90
//	TOTAL                            10                  $19.90
//	low stock: none
//
// Every line uses the format "%-8s %-20s %5s %10s %12s\n". Rows contain
// SKU, Name, Quantity, FormatCents(PriceCents) and FormatCents(Value()); the
// TOTAL line has the total quantity and total value with empty NAME and
// PRICE columns. The last line is "low stock: " followed by the low-stock
// SKUs in ascending order joined by ", ", or "none". Long values are not
// truncated. An unknown key returns an error wrapping ErrInvalid and writes
// nothing. The first write error (if any) is returned.
func (inv *Inventory) WriteReport(w io.Writer, key SortKey) error {
	return errors.New("not implemented") // TODO
}
