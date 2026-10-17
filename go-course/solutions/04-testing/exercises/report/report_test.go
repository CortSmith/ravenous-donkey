package report

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// update rewrites the golden files instead of comparing against them:
//
//	go test ./modules/04-testing/exercises/report/ -update
//
// ALWAYS read the rewritten files (git diff) before trusting them: -update
// happily records buggy output as "correct".
var update = flag.Bool("update", false, "rewrite golden files in testdata/")

func TestRenderGolden(t *testing.T) {
	tests := []struct {
		name  string
		items []Item
	}{
		{"empty", nil},
		{"basic", []Item{
			{Name: "Coffee", Qty: 2, PriceCents: 350},
			{Name: "Bagel", Qty: 1, PriceCents: 225},
		}},
		{"cents", []Item{ // amounts whose cents part is below 10
			{Name: "Gum", Qty: 1, PriceCents: 5},
			{Name: "Stamp", Qty: 3, PriceCents: 1205},
			{Name: "Free sample", Qty: 4, PriceCents: 0},
		}},
		{"unicode", []Item{ // multi-byte names must not widen the column
			{Name: "Crème brûlée", Qty: 1, PriceCents: 650},
			{Name: "Tea", Qty: 2, PriceCents: 275},
			{Name: "Café", Qty: 1, PriceCents: 300},
		}},
		{"wide_numbers", []Item{
			{Name: "Server", Qty: 12, PriceCents: 1234567},
			{Name: "Cable", Qty: 1000, PriceCents: 99},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Render(&buf, tt.items); err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			assertGolden(t, filepath.Join("testdata", tt.name+".golden"), buf.Bytes())
		})
	}
}

// assertGolden compares got with the golden file at path, or rewrites the
// file when -update is set.
func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s:\n%s", path, lineDiff(string(want), string(got)))
	}
}

// lineDiff is a tiny line-by-line diff: good enough for short golden files
// and dependency-free. Lines are quoted so whitespace differences show.
func lineDiff(want, got string) string {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	var b strings.Builder
	for i := range max(len(w), len(g)) {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl == gl {
			fmt.Fprintf(&b, "  %q\n", wl)
			continue
		}
		fmt.Fprintf(&b, "- %q (want)\n+ %q (got)\n", wl, gl)
	}
	return b.String()
}

// TestRenderLayoutRules checks the layout rules directly, independent of
// any golden file, so a wrongly updated golden file cannot hide them.
func TestRenderLayoutRules(t *testing.T) {
	items := []Item{
		{Name: "Crème brûlée", Qty: 1, PriceCents: 650},
		{Name: "Tea", Qty: 10, PriceCents: 5},
	}
	var buf bytes.Buffer
	if err := Render(&buf, items); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("output does not end in a newline: %q", out)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if got, want := len(lines), len(items)+4; got != want {
		t.Fatalf("got %d lines, want %d:\n%s", got, want, out)
	}
	for i, line := range lines {
		if strings.TrimRight(line, " ") != line {
			t.Errorf("line %d has trailing spaces: %q", i, line)
		}
	}
	// "Crème brûlée" is 12 runes, so the ITEM rule is 12 dashes.
	if want := "------------  "; !strings.HasPrefix(lines[1], want) {
		t.Errorf("rule line = %q, want prefix %q (ITEM column 12 runes wide)", lines[1], want)
	}
	// 12-rune ITEM column, then "  " + 3-wide QTY, "  " + 5-wide PRICE,
	// "  " + 6-wide AMOUNT.
	if want := "Tea" + strings.Repeat(" ", 9) + "   10   0.05    0.50"; lines[3] != want {
		t.Errorf("Tea line = %q, want %q", lines[3], want)
	}
	if want := "TOTAL" + strings.Repeat(" ", 7) + strings.Repeat(" ", 2+3+2+5+2+2) + "7.00"; lines[5] != want {
		t.Errorf("TOTAL line = %q, want %q", lines[5], want)
	}
}

func TestMoney(t *testing.T) {
	tests := []struct {
		cents int64
		want  string
	}{
		{0, "0.00"},
		{5, "0.05"},
		{10, "0.10"},
		{99, "0.99"},
		{100, "1.00"},
		{1205, "12.05"},
		{123456789, "1234567.89"},
	}
	for _, tt := range tests {
		if got := money(tt.cents); got != tt.want {
			t.Errorf("money(%d) = %q, want %q", tt.cents, got, tt.want)
		}
	}
}

// errWriter fails every write, to check that Render reports write errors.
type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRenderReturnsWriteError(t *testing.T) {
	errDisk := errors.New("disk full")
	err := Render(errWriter{errDisk}, []Item{{Name: "x", Qty: 1, PriceCents: 1}})
	if !errors.Is(err, errDisk) {
		t.Errorf("Render() error = %v, want %v", err, errDisk)
	}
}

func ExampleRender() {
	_ = Render(os.Stdout, []Item{
		{Name: "Coffee", Qty: 2, PriceCents: 350},
		{Name: "Bagel", Qty: 1, PriceCents: 225},
	})
	// Output:
	// ITEM    QTY  PRICE  AMOUNT
	// ------  ---  -----  ------
	// Coffee    2   3.50    7.00
	// Bagel     1   2.25    2.25
	// ------  ---  -----  ------
	// TOTAL                 9.25
}
