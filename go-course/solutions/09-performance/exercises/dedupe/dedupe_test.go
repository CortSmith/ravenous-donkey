package dedupe

import (
	"slices"
	"strconv"
	"testing"
)

func TestUnique(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  []string
	}{
		{"nil", nil, nil},
		{"empty", []string{}, nil},
		{"single", []string{"a"}, []string{"a"}},
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"all same", []string{"x", "x", "x"}, []string{"x"}},
		{"keeps first occurrence order", []string{"b", "a", "b", "c", "a"}, []string{"b", "a", "c"}},
		{"empty string is a value", []string{"", "a", "", "a"}, []string{"", "a"}},
		{"case sensitive", []string{"Go", "go", "GO", "go"}, []string{"Go", "go", "GO"}},
		{"unicode", []string{"é", "é", "é"}, []string{"é", "é"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := slices.Clone(tt.items)
			got := Unique(in)
			if len(got) != len(tt.want) || (len(got) > 0 && !slices.Equal(got, tt.want)) {
				t.Errorf("Unique(%q) = %q, want %q", tt.items, got, tt.want)
			}
			if !slices.Equal(in, tt.items) {
				t.Errorf("Unique modified its input: got %q, want %q", in, tt.items)
			}
		})
	}
}

func TestUniqueLarge(t *testing.T) {
	items := testItems(5000, 1000)
	got := Unique(items)
	if len(got) != 1000 {
		t.Fatalf("Unique(5000 items, 1000 distinct) returned %d items, want 1000", len(got))
	}
	for i, s := range got {
		if want := "item-" + strconv.Itoa(i); s != want {
			t.Fatalf("Unique(...)[%d] = %q, want %q (first-occurrence order)", i, s, want)
		}
	}
}

func TestUniqueAllocs(t *testing.T) {
	items := testItems(2000, 1000)
	allocs := testing.AllocsPerRun(50, func() {
		if got := Unique(items); len(got) != 1000 {
			t.Fatalf("Unique returned %d items, want 1000", len(got))
		}
	})
	const maxAllocs = 12
	if allocs > maxAllocs {
		t.Errorf("Unique(2000 items, 1000 distinct) allocates %.0f times per call, want <= %d.\n"+
			"Hint: make(map[K]V, hint) and make([]T, 0, capacity) let the runtime allocate "+
			"the right size once instead of growing step by step.", allocs, maxAllocs)
	}
}

// testItems returns n strings drawn from "item-0".."item-(distinct-1)", in
// which item-k first appears before item-(k+1).
func testItems(n, distinct int) []string {
	names := make([]string, distinct)
	for i := range names {
		names[i] = "item-" + strconv.Itoa(i)
	}
	items := make([]string, n)
	for i := range items {
		if i%2 == 0 || i/2 >= distinct {
			items[i] = names[(i/2)%distinct]
		} else {
			items[i] = names[(i*7)%(i/2+1)] // a repeat of something already seen
		}
	}
	return items
}

func BenchmarkUnique(b *testing.B) {
	items := testItems(2000, 1000)
	b.ReportAllocs()
	for b.Loop() {
		Unique(items)
	}
}
