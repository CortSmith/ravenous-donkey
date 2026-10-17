# Inventory Manager

## Overview

The Inventory Manager is a business-logic layer for managing product stock. You'll implement:

1. **Product**: a value type representing a stock-keeping unit (SKU) with name, price, quantity and reorder level.
2. **Store**: an interface for persisting products, with an in-memory implementation.
3. **Inventory**: a service layer on top of the store that sells, restocks, filters, sorts and reports products.
4. **Alerter**: an interface for notifying when products enter low-stock status.

This project exercises struct methods, interfaces, embedding, error handling, sorting and threading safety.

## Requirements

### Product Type

`Product` is a value type with fields: `SKU`, `Name`, `Category`, `PriceCents`, `Quantity`, `ReorderLevel`.

- **SKU** (string): a unique identifier, never empty.
- **Name** (string): the product's name, never empty.
- **PriceCents** (int64): the price in cents (not dollars). Always non-negative. Use int64 to avoid floating-point precision issues.
- **Quantity** (int): units in stock, non-negative.
- **ReorderLevel** (int): the threshold below which a product is "low stock", non-negative.
- **Category** (string): the product's category (may be empty).

Implement:

- **`Validate() error`**: validates all fields. Return an error wrapping `ErrInvalid` if any field is invalid. The error message must mention the offending field name. Check fields in this order: SKU, Name, PriceCents, Quantity, ReorderLevel.

- **`Value() int64`**: returns the total stock value (PriceCents × Quantity). Mind integer overflow for very large quantities.

- **`LowStock() bool`**: returns true if Quantity <= ReorderLevel.

- **`String() string`**: formats the product as `` `SKU "Name" qty=N @ $X.YY` ``, e.g., `` `A-1 "Widget" qty=3 @ $1.99` ``. Use the `FormatCents` function to format prices.

- **`FormatCents(cents int64) string`** (package function): formats an amount of cents as a dollar string. Examples:
  - 199 → "$1.99"
  - 5 → "$0.05"
  - -250 → "-$2.50"
  - 123456 → "$1234.56"
  - 0 → "$0.00"
  - Math.MinInt64 and other edge cases must be handled correctly.

### Store Interface and MemoryStore Implementation

The `Store` interface defines CRUD operations for products:

```go
// Checks the interface against YOUR MemoryStore (project/store.go).
import inventory "gocourse/modules/03-types-interfaces/project"

type Product = inventory.Product

type Store interface {
	Add(p Product) error
	Get(sku string) (Product, error)
	Update(p Product) error
	Delete(sku string) error
	List() []Product
}

var s Store = inventory.NewMemoryStore() // compiles: the method set matches
err := s.Add(Product{SKU: "A-1", Name: "Widget", PriceCents: 199, Quantity: 10})
fmt.Println("Add:", err, "List:", s.List()) // "not implemented" until you write it
```

- **`Add(p Product) error`**: inserts a new product. Return an error wrapping `ErrDuplicate` if the SKU already exists.

- **`Get(sku string) (Product, error)`**: returns the product with the given SKU. Return an error wrapping `ErrNotFound` if not found.

- **`Update(p Product) error`**: replaces the product with the given SKU. Return an error wrapping `ErrNotFound` if the SKU doesn't exist.

- **`Delete(sku string) error`**: removes the product. Return an error wrapping `ErrNotFound` if not found.

- **`List() []Product`**: returns all products sorted by SKU (ascending). Return a newly allocated slice (not the internal backing store).

**MemoryStore** is an in-memory implementation. It must:

- Use a map to store products by SKU.
- Protect the map with a `sync.Mutex` for concurrent safety.
- Return copies of products, never references to internal state.
- Add a compile-time assertion proving `*MemoryStore` implements `Store`.

### Alerter Interface

The `Alerter` interface notifies when a product enters low-stock status:

```go
import inventory "gocourse/modules/03-types-interfaces/project"

type Product = inventory.Product

type Alerter interface {
	LowStock(p Product)
}

var _ Alerter = inventory.AlerterFunc(nil) // YOUR AlerterFunc satisfies it

var a Alerter = inventory.AlerterFunc(func(p Product) { fmt.Println("low stock:", p.SKU) })
a.LowStock(Product{SKU: "A-1"}) // low stock: A-1 (once your AlerterFunc calls f)
```

Implement **`AlerterFunc`**: a function adapter like `http.HandlerFunc`. It should allow a function to satisfy the `Alerter` interface.

```go
type Product struct {
	SKU      string
	Quantity int
}

type AlerterFunc func(Product)

func (f AlerterFunc) LowStock(p Product) {
	f(p)
}

// Usage:
alerter := AlerterFunc(func(p Product) {
	fmt.Printf("Low stock: %v\n", p)
})
alerter.LowStock(Product{SKU: "A-1", Quantity: 2}) // Low stock: {A-1 2}
```

### Inventory Service

The `Inventory` type is a service layer on top of a `Store`. It manages products and optionally notifies an `Alerter` when products become low stock.

**Concurrency note**: the `Store` is concurrent-safe, but the `Inventory` methods (`Sell`, `Receive`) do a Get-then-Update. This sequence is not atomic—a concurrent sale could lose updates. Add a `sync.Mutex` to the `Inventory` to make Sell/Receive atomic.

Implement:

- **`New(store Store, alerter Alerter) *Inventory`**: construct an Inventory. The alerter may be nil (no alerts).

- **`AddProduct(p Product) error`**: validates the product and adds it to the store. Return the error from `Validate()` or `store.Add()`.

- **`Receive(sku string, qty int) error`**: adds `qty` units to a product's quantity. Errors:
  - `qty` must be positive (else an error wrapping `ErrInvalid` mentioning "quantity").
  - SKU must exist (else `store.Get()` error).
  - Never triggers an alert (even if it moves the product into low stock after receiving, don't alert).

- **`Sell(sku string, qty int) error`**: removes `qty` units from a product's quantity. Errors:
  - `qty` must be positive (else an error wrapping `ErrInvalid` mentioning "quantity").
  - SKU must exist (else `store.Get()` error).
  - If selling `qty` exceeds the current quantity, return an error wrapping `ErrInsufficientStock` that mentions the SKU, and change nothing.
  - If the sale moves the product from not-low-stock to low-stock (before the sale `!LowStock()`, after the sale `LowStock()`), call the alerter (if any) with the updated product.

- **`Filter(keep func(Product) bool) []Product`**: returns products for which the filter function returns true, sorted by SKU. Return an empty (non-nil) slice if no products match.

- **`LowStock() []Product`**: returns low-stock products sorted by SKU. Implement as a one-liner using `Filter`.

- **`TotalValue() int64`**: returns the sum of `Value()` over all products.

- **`Sorted(key SortKey) ([]Product, error)`**: returns all products in the order specified by the sort key. Errors:
  - Unknown `SortKey` returns an error wrapping `ErrInvalid`.
  - Orders:
    - `BySKU`: ascending SKU.
    - `ByName`: ascending Name.
    - `ByValue`: descending Value (highest first).
    - `ByQuantity`: ascending Quantity.
  - Ties are always broken by ascending SKU for deterministic output.

- **`SortKey.String() string`**: returns the lowercase name of the key ("sku", "name", "value", "quantity") or "SortKey(N)" for unknown values.

- **`WriteReport(w io.Writer, key SortKey) error`**: writes a formatted stock report to `w`. Format and layout:

```
SKU      NAME                   QTY      PRICE        VALUE
A-1      Widget                  10      $1.99       $19.90
A-2      Gadget                  20      $0.50       $10.00
TOTAL                            30                  $29.90
low stock: A-1, A-2
```

- Each line follows the format: `"%-8s %-20s %5s %10s %12s\n"` (left-aligned SKU and NAME, right-aligned the rest).
- Rows: SKU, Name, Quantity (as string), FormatCents(PriceCents), FormatCents(Value()).
- TOTAL line: empty SKU, empty NAME, total quantity, empty PRICE, total value. Use spaces or empty strings for the NAME and PRICE columns.
- Last line: `"low stock: "` followed by low-stock SKUs in ascending order, comma-separated. If no products are low stock, use `"none"`.
- Unknown sort keys return an error wrapping `ErrInvalid` and write nothing.
- The first write error (if any) is returned.

## Acceptance Criteria

- [ ] All types (`Product`, `Store`, `Inventory`) are defined and compile.
- [ ] All methods are implemented and tested.
- [ ] Errors wrap sentinel errors with `%w` where specified.
- [ ] `Product.Validate()` checks fields in the specified order.
- [ ] `MemoryStore` is concurrent-safe (uses `sync.Mutex`).
- [ ] `Inventory.Sell()` and `Receive()` are atomic (use `sync.Mutex`).
- [ ] `Inventory.Sell()` alerts only when moving from not-low to low stock.
- [ ] `Sorted()` and `WriteReport()` produce deterministic output (ties broken by SKU).
- [ ] `FormatCents` handles all int64 values correctly (including `math.MinInt64`).
- [ ] All tests pass: `go test ./modules/03-types-interfaces/project/`.
- [ ] Code is gofmt-clean and passes `go vet`.

## Stretch Goals

- Add a `Delete(sku string) error` method to `Inventory` that removes a product and any associated alerts.
- Add an `Audit(before, after time.Time) ([]Transaction, error)` method that logs and returns a history of changes (requires a transaction log in `MemoryStore`).
- Implement a second `Store` backend, e.g., using a slice instead of a map (good for small inventories that are frequently iterated).

## Testing

The test file includes table-driven tests for all Product methods, Store operations, Inventory business logic, sorting, reporting, and alerting. Tests verify:

- Correct computation of area, perimeter, formatting.
- Proper error handling and wrapping.
- Concurrent safety (with a race detector check).
- Deterministic sorting and output.
- Alerter behavior.

**Run the project tests with:** `go test ./modules/03-types-interfaces/project/`

**Run with the race detector:** `go test -race ./modules/03-types-interfaces/project/`
