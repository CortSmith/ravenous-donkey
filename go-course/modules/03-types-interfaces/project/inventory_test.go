package inventory

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
)

var _ Alerter = AlerterFunc(nil)

// alertRecorder is a hand-written fake: the consumer-defined Alerter
// interface makes test doubles trivial, no mocking library needed.
type alertRecorder struct {
	mu   sync.Mutex
	got  []Product
	skus []string
}

func (r *alertRecorder) LowStock(p Product) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, p)
	r.skus = append(r.skus, p.SKU)
}

func sampleProducts() []Product {
	return []Product{
		{SKU: "B-2", Name: "Bolt", Category: "hardware", PriceCents: 25, Quantity: 400, ReorderLevel: 100},
		{SKU: "A-1", Name: "Widget", Category: "tools", PriceCents: 199, Quantity: 10, ReorderLevel: 3},
		{SKU: "D-4", Name: "Drill", Category: "tools", PriceCents: 4999, Quantity: 2, ReorderLevel: 2},
		{SKU: "C-3", Name: "Anvil", Category: "tools", PriceCents: 10000, Quantity: 1, ReorderLevel: 0},
	}
}

func newTestInventory(t *testing.T, alerter Alerter) *Inventory {
	t.Helper()
	inv := New(NewMemoryStore(), alerter)
	for _, p := range sampleProducts() {
		if err := inv.AddProduct(p); err != nil {
			t.Fatalf("AddProduct(%v) = %v", p, err)
		}
	}
	return inv
}

func skus(ps []Product) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.SKU
	}
	return out
}

func quantity(t *testing.T, inv *Inventory, sku string) int {
	t.Helper()
	for _, p := range inv.Filter(func(p Product) bool { return p.SKU == sku }) {
		return p.Quantity
	}
	t.Fatalf("product %s not found", sku)
	return 0
}

func TestAddProductValidates(t *testing.T) {
	inv := New(NewMemoryStore(), nil)
	if err := inv.AddProduct(Product{SKU: "X"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("AddProduct(no name) = %v, want ErrInvalid", err)
	}
	p := Product{SKU: "X", Name: "x"}
	if err := inv.AddProduct(p); err != nil {
		t.Fatalf("AddProduct(%v) = %v", p, err)
	}
	if err := inv.AddProduct(p); !errors.Is(err, ErrDuplicate) {
		t.Errorf("duplicate AddProduct = %v, want ErrDuplicate", err)
	}
}

func TestReceive(t *testing.T) {
	inv := newTestInventory(t, nil)
	if err := inv.Receive("A-1", 5); err != nil {
		t.Fatalf("Receive(A-1, 5) = %v", err)
	}
	if got := quantity(t, inv, "A-1"); got != 15 {
		t.Errorf("A-1 quantity = %d after receiving 5, want 15", got)
	}
	for _, qty := range []int{0, -3} {
		if err := inv.Receive("A-1", qty); !errors.Is(err, ErrInvalid) {
			t.Errorf("Receive(A-1, %d) = %v, want ErrInvalid", qty, err)
		}
	}
	if err := inv.Receive("nope", 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Receive(nope, 1) = %v, want ErrNotFound", err)
	}
	if got := quantity(t, inv, "A-1"); got != 15 {
		t.Errorf("A-1 quantity = %d after failed receives, want 15", got)
	}
}

func TestSell(t *testing.T) {
	inv := newTestInventory(t, nil)
	if err := inv.Sell("A-1", 4); err != nil {
		t.Fatalf("Sell(A-1, 4) = %v", err)
	}
	if got := quantity(t, inv, "A-1"); got != 6 {
		t.Errorf("A-1 quantity = %d, want 6", got)
	}
	if err := inv.Sell("A-1", 6); err != nil {
		t.Fatalf("selling the exact remaining stock: Sell(A-1, 6) = %v", err)
	}
	if got := quantity(t, inv, "A-1"); got != 0 {
		t.Errorf("A-1 quantity = %d, want 0", got)
	}

	err := inv.Sell("B-2", 401)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Errorf("Sell(B-2, 401) = %v, want ErrInsufficientStock", err)
	}
	if err != nil && !strings.Contains(err.Error(), "B-2") {
		t.Errorf("Sell error %q should mention the SKU", err)
	}
	if got := quantity(t, inv, "B-2"); got != 400 {
		t.Errorf("B-2 quantity = %d after failed sale, want 400 (failed sales change nothing)", got)
	}
	for _, qty := range []int{0, -1} {
		if err := inv.Sell("B-2", qty); !errors.Is(err, ErrInvalid) {
			t.Errorf("Sell(B-2, %d) = %v, want ErrInvalid", qty, err)
		}
	}
	if err := inv.Sell("nope", 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Sell(nope, 1) = %v, want ErrNotFound", err)
	}
}

func TestSellAlerts(t *testing.T) {
	rec := &alertRecorder{}
	inv := newTestInventory(t, rec)

	// A-1: qty 10, reorder at 3.
	steps := []struct {
		sku       string
		qty       int
		wantAlert bool
	}{
		{"A-1", 5, false}, // 10 -> 5: still above 3
		{"A-1", 2, true},  // 5 -> 3: crosses into low stock (3 <= 3)
		{"A-1", 1, false}, // 3 -> 2: already low, no repeat alert
		{"B-2", 1, false}, // 400 -> 399
		{"D-4", 1, false}, // already low before the sale (2 <= 2)
	}
	for _, st := range steps {
		before := len(rec.got)
		if err := inv.Sell(st.sku, st.qty); err != nil {
			t.Fatalf("Sell(%s, %d) = %v", st.sku, st.qty, err)
		}
		if alerted := len(rec.got) > before; alerted != st.wantAlert {
			t.Errorf("Sell(%s, %d): alerted = %v, want %v", st.sku, st.qty, alerted, st.wantAlert)
		}
	}
	if len(rec.got) == 1 && rec.got[0].Quantity != 3 {
		t.Errorf("alert carried Quantity %d, want the updated quantity 3", rec.got[0].Quantity)
	}

	// Receiving stock never alerts; after a restock, a new crossing alerts again.
	if err := inv.Receive("A-1", 8); err != nil { // 2 -> 10
		t.Fatal(err)
	}
	if err := inv.Sell("A-1", 7); err != nil { // 10 -> 3
		t.Fatal(err)
	}
	if want := []string{"A-1", "A-1"}; !slices.Equal(rec.skus, want) {
		t.Errorf("alerts = %v, want %v", rec.skus, want)
	}
}

func TestSellWithAlerterFuncAndNil(t *testing.T) {
	var got []string
	inv := newTestInventory(t, AlerterFunc(func(p Product) { got = append(got, p.SKU) }))
	if err := inv.Sell("A-1", 7); err != nil {
		t.Fatal(err)
	}
	if want := []string{"A-1"}; !slices.Equal(got, want) {
		t.Errorf("AlerterFunc received %v, want %v", got, want)
	}

	noAlerts := newTestInventory(t, nil)
	if err := noAlerts.Sell("A-1", 7); err != nil { // must not panic with a nil Alerter
		t.Fatal(err)
	}
}

// Run with -race. Without a lock around Get+Update, concurrent sales lose
// updates and the final quantity is wrong.
func TestConcurrentSells(t *testing.T) {
	inv := New(NewMemoryStore(), nil)
	if err := inv.AddProduct(Product{SKU: "S", Name: "s", Quantity: 200}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := inv.Sell("S", 2); err != nil {
				t.Errorf("Sell = %v", err)
			}
		}()
	}
	wg.Wait()
	if got := quantity(t, inv, "S"); got != 0 {
		t.Errorf("quantity after 100 concurrent sales of 2 from 200 = %d, want 0 (lost updates?)", got)
	}
	if err := inv.Sell("S", 1); !errors.Is(err, ErrInsufficientStock) {
		t.Errorf("Sell on empty stock = %v, want ErrInsufficientStock", err)
	}
}

func TestFilterAndLowStock(t *testing.T) {
	inv := newTestInventory(t, nil)
	tools := inv.Filter(func(p Product) bool { return p.Category == "tools" })
	if got, want := skus(tools), []string{"A-1", "C-3", "D-4"}; !slices.Equal(got, want) {
		t.Errorf("Filter(tools) = %v, want %v", got, want)
	}
	none := inv.Filter(func(Product) bool { return false })
	if none == nil || len(none) != 0 {
		t.Errorf("Filter(nothing) = %#v, want empty non-nil slice", none)
	}
	if got, want := skus(inv.LowStock()), []string{"D-4"}; !slices.Equal(got, want) {
		t.Errorf("LowStock() = %v, want %v", got, want)
	}
	// C-3: qty 1, reorder 0 -> not low. Sell it out -> low.
	if err := inv.Sell("C-3", 1); err != nil {
		t.Fatal(err)
	}
	if got, want := skus(inv.LowStock()), []string{"C-3", "D-4"}; !slices.Equal(got, want) {
		t.Errorf("LowStock() = %v, want %v", got, want)
	}
}

func TestTotalValue(t *testing.T) {
	if got := New(NewMemoryStore(), nil).TotalValue(); got != 0 {
		t.Errorf("TotalValue() of empty inventory = %d, want 0", got)
	}
	inv := newTestInventory(t, nil)
	// 400*25 + 10*199 + 2*4999 + 1*10000
	if got, want := inv.TotalValue(), int64(10000+1990+9998+10000); got != want {
		t.Errorf("TotalValue() = %d, want %d", got, want)
	}
}

func TestSortKeyString(t *testing.T) {
	tests := []struct {
		k    SortKey
		want string
	}{
		{BySKU, "sku"}, {ByName, "name"}, {ByValue, "value"}, {ByQuantity, "quantity"},
		{SortKey(42), "SortKey(42)"}, {SortKey(-1), "SortKey(-1)"},
	}
	for _, tt := range tests {
		if got := tt.k.String(); got != tt.want {
			t.Errorf("SortKey(%d).String() = %q, want %q", int(tt.k), got, tt.want)
		}
	}
}

func TestSorted(t *testing.T) {
	inv := newTestInventory(t, nil)
	// Add ties to check the SKU tie-break.
	extra := []Product{
		{SKU: "E-5", Name: "Bolt", PriceCents: 1, Quantity: 2},    // same name as B-2, same qty as D-4
		{SKU: "0-0", Name: "Zed", PriceCents: 10000, Quantity: 1}, // same value as C-3 and B-2
	}
	for _, p := range extra {
		if err := inv.AddProduct(p); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		key  SortKey
		want []string
	}{
		{BySKU, []string{"0-0", "A-1", "B-2", "C-3", "D-4", "E-5"}},
		{ByName, []string{"C-3", "B-2", "E-5", "D-4", "A-1", "0-0"}},
		// values: B-2 10000, A-1 1990, D-4 9998, C-3 10000, E-5 2, 0-0 10000
		{ByValue, []string{"0-0", "B-2", "C-3", "D-4", "A-1", "E-5"}},
		// quantities: B-2 400, A-1 10, D-4 2, C-3 1, E-5 2, 0-0 1
		{ByQuantity, []string{"0-0", "C-3", "D-4", "E-5", "A-1", "B-2"}},
	}
	for _, tt := range tests {
		t.Run(tt.key.String(), func(t *testing.T) {
			got, err := inv.Sorted(tt.key)
			if err != nil {
				t.Fatalf("Sorted(%v) error = %v", tt.key, err)
			}
			if !slices.Equal(skus(got), tt.want) {
				t.Errorf("Sorted(%v) = %v, want %v", tt.key, skus(got), tt.want)
			}
		})
	}
	if _, err := inv.Sorted(SortKey(99)); !errors.Is(err, ErrInvalid) {
		t.Errorf("Sorted(SortKey(99)) error = %v, want ErrInvalid", err)
	}
}

const wantReportByValue = `SKU      NAME                   QTY      PRICE        VALUE
B-2      Bolt                   400      $0.25      $100.00
C-3      Anvil                    1    $100.00      $100.00
D-4      Drill                    2     $49.99       $99.98
A-1      Widget                  10      $1.99       $19.90
TOTAL                           413                 $319.88
low stock: D-4
`

func TestWriteReport(t *testing.T) {
	inv := newTestInventory(t, nil)
	var sb strings.Builder
	if err := inv.WriteReport(&sb, ByValue); err != nil {
		t.Fatalf("WriteReport error = %v", err)
	}
	if got := sb.String(); got != wantReportByValue {
		t.Errorf("WriteReport(ByValue) output mismatch\n--- got ---\n%s--- want ---\n%s", got, wantReportByValue)
	}
}

func TestWriteReportLowStockSortedBySKU(t *testing.T) {
	inv := newTestInventory(t, nil)
	if err := inv.Sell("C-3", 1); err != nil { // C-3 becomes low
		t.Fatal(err)
	}
	var sb strings.Builder
	if err := inv.WriteReport(&sb, ByQuantity); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(sb.String(), "\n"), "\n")
	if got, want := lines[len(lines)-1], "low stock: C-3, D-4"; got != want {
		t.Errorf("last line = %q, want %q", got, want)
	}
	if got, want := len(lines), 1+4+1+1; got != want {
		t.Errorf("report has %d lines, want %d (header, 4 rows, total, low stock)", got, want)
	}
}

func TestWriteReportEmpty(t *testing.T) {
	inv := New(NewMemoryStore(), nil)
	var sb strings.Builder
	if err := inv.WriteReport(&sb, BySKU); err != nil {
		t.Fatal(err)
	}
	want := "SKU      NAME                   QTY      PRICE        VALUE\n" +
		"TOTAL                             0                   $0.00\n" +
		"low stock: none\n"
	if got := sb.String(); got != want {
		t.Errorf("empty report:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestWriteReportBadKey(t *testing.T) {
	inv := newTestInventory(t, nil)
	var sb strings.Builder
	if err := inv.WriteReport(&sb, SortKey(7)); !errors.Is(err, ErrInvalid) {
		t.Errorf("WriteReport(SortKey(7)) = %v, want ErrInvalid", err)
	}
	if sb.Len() != 0 {
		t.Errorf("WriteReport with a bad key wrote %q, want nothing", sb.String())
	}
}

// failingWriter fails once it has accepted `after` writes.
type failingWriter struct{ after, n int }

var errDiskFull = errors.New("disk full")

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.n >= w.after {
		return 0, errDiskFull
	}
	w.n++
	return len(p), nil
}

func TestWriteReportPropagatesWriteErrors(t *testing.T) {
	inv := newTestInventory(t, nil)
	for after := range 7 {
		w := &failingWriter{after: after}
		if err := inv.WriteReport(w, BySKU); !errors.Is(err, errDiskFull) {
			t.Errorf("writer failing after %d writes: WriteReport = %v, want errDiskFull", after, err)
		}
	}
}
