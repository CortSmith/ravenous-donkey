package dedupe

import (
	"fmt"
	"slices"
	"testing"
)

func TestFastVariantsMatch(t *testing.T) {
	var d Deduper
	for _, n := range []int{0, 1, 5, smallN, smallN + 1, 300} {
		items := testItems(n, max(n/2, 1))
		want := Unique(items)
		if got := UniqueFast(items); !slices.Equal(got, want) {
			t.Errorf("UniqueFast(%d items) = %q, want %q", n, got, want)
		}
		if got := d.AppendUnique(nil, items); !slices.Equal(got, want) {
			t.Errorf("Deduper.AppendUnique(%d items) = %q, want %q", n, got, want)
		}
	}
}

func TestDeduperSteadyStateAllocs(t *testing.T) {
	items := testItems(2000, 1000)
	var d Deduper
	dst := make([]string, 0, len(items))
	dst = d.AppendUnique(dst, items) // warm up the map
	allocs := testing.AllocsPerRun(50, func() {
		dst = d.AppendUnique(dst[:0], items)
	})
	if allocs != 0 {
		t.Errorf("Deduper.AppendUnique steady state allocates %.0f times, want 0", allocs)
	}
}

func BenchmarkUniqueSmall(b *testing.B) {
	for _, n := range []int{4, 8, 16, 32, 64} {
		items := testItems(n, n/2)
		b.Run(fmt.Sprintf("map/n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				Unique(items)
			}
		})
		b.Run(fmt.Sprintf("scan/n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				out := make([]string, 0, len(items))
				for _, it := range items {
					if !slices.Contains(out, it) {
						out = append(out, it)
					}
				}
			}
		})
	}
}

func BenchmarkDeduper(b *testing.B) {
	items := testItems(2000, 1000)
	var d Deduper
	var dst []string
	b.ReportAllocs()
	for b.Loop() {
		dst = d.AppendUnique(dst[:0], items)
	}
}
