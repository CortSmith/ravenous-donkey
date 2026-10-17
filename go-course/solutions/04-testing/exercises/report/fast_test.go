package report

import (
	"bytes"
	"testing"
)

// FuzzRenderFastMatchesRender is a differential test: Render is the oracle
// for RenderFast. Each fuzz input becomes a two-item bill.
//
//	go test ./solutions/04-testing/exercises/report/ -run='^$' -fuzz=FuzzRenderFastMatchesRender -fuzztime=20s
func FuzzRenderFastMatchesRender(f *testing.F) {
	f.Add("Coffee", uint16(2), uint32(350), "Crème brûlée", uint16(1), uint32(5))
	f.Add("", uint16(0), uint32(0), "x", uint16(1000), uint32(1234567))
	f.Fuzz(func(t *testing.T, n1 string, q1 uint16, p1 uint32, n2 string, q2 uint16, p2 uint32) {
		items := []Item{
			{Name: n1, Qty: int(q1), PriceCents: int64(p1)},
			{Name: n2, Qty: int(q2), PriceCents: int64(p2)},
		}
		for _, in := range [][]Item{nil, items[:1], items} {
			var want, got bytes.Buffer
			if err := Render(&want, in); err != nil {
				t.Fatal(err)
			}
			if err := RenderFast(&got, in); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got.Bytes(), want.Bytes()) {
				t.Fatalf("RenderFast(%+v) differs from Render:\n%s", in, lineDiff(want.String(), got.String()))
			}
		}
	})
}

var benchItems = []Item{
	{Name: "Coffee", Qty: 2, PriceCents: 350},
	{Name: "Bagel", Qty: 1, PriceCents: 225},
	{Name: "Crème brûlée", Qty: 1, PriceCents: 650},
	{Name: "Server", Qty: 12, PriceCents: 1234567},
	{Name: "Cable", Qty: 1000, PriceCents: 99},
	{Name: "Gum", Qty: 1, PriceCents: 5},
}

func BenchmarkRender(b *testing.B) {
	var buf bytes.Buffer
	b.Run("Readable", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			buf.Reset()
			Render(&buf, benchItems)
		}
	})
	b.Run("Fast", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			buf.Reset()
			RenderFast(&buf, benchItems)
		}
	})
}
