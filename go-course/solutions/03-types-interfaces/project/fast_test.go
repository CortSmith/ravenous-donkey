package inventory

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func bigInventory(tb testing.TB, n int) *Inventory {
	tb.Helper()
	inv := New(NewMemoryStore(), nil)
	for i := range n {
		p := Product{
			SKU:          fmt.Sprintf("SKU-%05d", i),
			Name:         fmt.Sprintf("Product number %d", i),
			PriceCents:   int64(i * 37 % 10000),
			Quantity:     i * 7 % 300,
			ReorderLevel: 20,
		}
		if i%2 == 0 {
			p.PriceCents = int64(i)
		}
		if err := inv.AddProduct(p); err != nil {
			tb.Fatal(err)
		}
	}
	return inv
}

func TestWriteReportFastMatches(t *testing.T) {
	invs := map[string]*Inventory{
		"empty":  New(NewMemoryStore(), nil),
		"sample": newTestInventory(t, nil),
		"big":    bigInventory(t, 500),
	}
	unicode := New(NewMemoryStore(), nil)
	for _, p := range []Product{
		{SKU: "Ü-1", Name: "Größenverstellbarer Schraubenschlüssel", PriceCents: 1234, Quantity: 3},
		{SKU: "VERY-LONG-SKU", Name: "日本", PriceCents: 7, Quantity: 0},
	} {
		if err := unicode.AddProduct(p); err != nil {
			t.Fatal(err)
		}
	}
	invs["unicode and overflow"] = unicode

	for name, inv := range invs {
		for _, key := range []SortKey{BySKU, ByName, ByValue, ByQuantity} {
			var a, b strings.Builder
			if err := inv.WriteReport(&a, key); err != nil {
				t.Fatal(err)
			}
			if err := inv.WriteReportFast(&b, key); err != nil {
				t.Fatal(err)
			}
			if a.String() != b.String() {
				t.Errorf("%s/%v: WriteReportFast differs\n--- readable ---\n%s--- fast ---\n%s", name, key, a.String(), b.String())
			}
		}
	}
}

func BenchmarkWriteReport(b *testing.B) {
	inv := bigInventory(b, 1000)
	b.ReportAllocs()
	for b.Loop() {
		_ = inv.WriteReport(io.Discard, ByValue)
	}
}

func BenchmarkWriteReportFast(b *testing.B) {
	inv := bigInventory(b, 1000)
	b.ReportAllocs()
	for b.Loop() {
		_ = inv.WriteReportFast(io.Discard, ByValue)
	}
}
