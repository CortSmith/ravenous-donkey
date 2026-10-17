package inventory

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
)

// Alerter is notified when a sale pushes a product into low stock. It is
// defined here, next to its consumer, and is deliberately tiny.
type Alerter interface {
	LowStock(p Product)
}

// AlerterFunc adapts an ordinary function to the Alerter interface, in the
// same way http.HandlerFunc adapts functions to http.Handler.
type AlerterFunc func(Product)

// LowStock calls f(p).
func (f AlerterFunc) LowStock(p Product) { f(p) }

// Inventory is the business layer on top of a Store.
type Inventory struct {
	// mu serialises read-modify-write sequences (Get then Update) so that
	// concurrent Sell/Receive calls cannot lose updates. The Store's own
	// lock only protects individual calls.
	mu      sync.Mutex
	store   Store
	alerter Alerter
}

// New returns an Inventory backed by store. alerter may be nil.
func New(store Store, alerter Alerter) *Inventory {
	return &Inventory{store: store, alerter: alerter}
}

// AddProduct validates p and adds it to the store.
func (inv *Inventory) AddProduct(p Product) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return inv.store.Add(p)
}

// Receive adds qty units of stock to the product. qty must be positive
// (else an error wrapping ErrInvalid). It never triggers an alert.
func (inv *Inventory) Receive(sku string, qty int) error {
	if qty <= 0 {
		return fmt.Errorf("receive %s: quantity %d must be positive: %w", sku, qty, ErrInvalid)
	}
	inv.mu.Lock()
	defer inv.mu.Unlock()
	p, err := inv.store.Get(sku)
	if err != nil {
		return err
	}
	p.Quantity += qty
	return inv.store.Update(p)
}

// Sell removes qty units of stock. qty must be positive (else ErrInvalid);
// selling more than is in stock fails with an error wrapping
// ErrInsufficientStock and changes nothing. If the sale moves the product
// from not-low to low stock, the alerter (if any) is called once with the
// updated product.
func (inv *Inventory) Sell(sku string, qty int) error {
	if qty <= 0 {
		return fmt.Errorf("sell %s: quantity %d must be positive: %w", sku, qty, ErrInvalid)
	}
	inv.mu.Lock()
	p, err := inv.store.Get(sku)
	if err != nil {
		inv.mu.Unlock()
		return err
	}
	if qty > p.Quantity {
		inv.mu.Unlock()
		return fmt.Errorf("sell %d of %s: only %d in stock: %w", qty, sku, p.Quantity, ErrInsufficientStock)
	}
	wasLow := p.LowStock()
	p.Quantity -= qty
	err = inv.store.Update(p)
	inv.mu.Unlock()
	if err != nil {
		return err
	}
	// Call out to the alerter after unlocking: never hold a lock while
	// running code you don't control (it might call back into Inventory).
	if !wasLow && p.LowStock() && inv.alerter != nil {
		inv.alerter.LowStock(p)
	}
	return nil
}

// Filter returns the products for which keep returns true, sorted by SKU.
// It returns an empty (non-nil) slice when nothing matches.
func (inv *Inventory) Filter(keep func(Product) bool) []Product {
	out := []Product{}
	for _, p := range inv.store.List() {
		if keep(p) {
			out = append(out, p)
		}
	}
	return out
}

// LowStock returns the low-stock products sorted by SKU.
func (inv *Inventory) LowStock() []Product {
	return inv.Filter(Product.LowStock) // a method expression: func(Product) bool
}

// TotalValue returns the sum of Value() over all products, in cents.
func (inv *Inventory) TotalValue() int64 {
	var total int64
	for _, p := range inv.store.List() {
		total += p.Value()
	}
	return total
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
	switch k {
	case BySKU:
		return "sku"
	case ByName:
		return "name"
	case ByValue:
		return "value"
	case ByQuantity:
		return "quantity"
	}
	return fmt.Sprintf("SortKey(%d)", int(k))
}

// compare returns the comparison function for k, or nil for unknown keys.
func (k SortKey) compare() func(a, b Product) int {
	switch k {
	case BySKU:
		return func(a, b Product) int { return cmp.Compare(a.SKU, b.SKU) }
	case ByName:
		return func(a, b Product) int {
			return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.SKU, b.SKU))
		}
	case ByValue:
		return func(a, b Product) int {
			return cmp.Or(cmp.Compare(b.Value(), a.Value()), cmp.Compare(a.SKU, b.SKU))
		}
	case ByQuantity:
		return func(a, b Product) int {
			return cmp.Or(cmp.Compare(a.Quantity, b.Quantity), cmp.Compare(a.SKU, b.SKU))
		}
	}
	return nil
}

// Sorted returns all products in the order given by key. Unknown keys
// return an error wrapping ErrInvalid.
func (inv *Inventory) Sorted(key SortKey) ([]Product, error) {
	compare := key.compare()
	if compare == nil {
		return nil, fmt.Errorf("unknown sort key %v: %w", key, ErrInvalid)
	}
	ps := inv.store.List()
	slices.SortFunc(ps, compare)
	return ps, nil
}

// reportLine is the format shared by the header, rows and total line.
const reportLine = "%-8s %-20s %5s %10s %12s\n"

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
// truncated. The first write error (if any) is returned.
func (inv *Inventory) WriteReport(w io.Writer, key SortKey) error {
	ps, err := inv.Sorted(key)
	if err != nil {
		return err
	}
	ew := &errWriter{w: w}
	ew.printf(reportLine, "SKU", "NAME", "QTY", "PRICE", "VALUE")
	var totalQty int
	var totalValue int64
	var low []string
	for _, p := range ps {
		ew.printf(reportLine, p.SKU, p.Name, fmt.Sprint(p.Quantity), FormatCents(p.PriceCents), FormatCents(p.Value()))
		totalQty += p.Quantity
		totalValue += p.Value()
		if p.LowStock() {
			low = append(low, p.SKU)
		}
	}
	ew.printf(reportLine, "TOTAL", "", fmt.Sprint(totalQty), "", FormatCents(totalValue))
	slices.Sort(low) // rows may be in another order
	lowStr := "none"
	if len(low) > 0 {
		lowStr = strings.Join(low, ", ")
	}
	ew.printf("low stock: %s\n", lowStr)
	return ew.err
}

// errWriter remembers the first write error and turns later writes into
// no-ops, so the report code can stay linear (see "Errors are values",
// https://go.dev/blog/errors-are-values).
type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) printf(format string, args ...any) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintf(ew.w, format, args...)
}
