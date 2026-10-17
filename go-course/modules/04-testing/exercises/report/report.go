// Package report renders an itemised bill as a plain-text table.
//
// This package has 3 bugs. Write golden-file tests in report_test.go that
// expose them, check the golden files by eye, then fix the code.
package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Item is one line of a bill. Qty and PriceCents must not be negative.
type Item struct {
	Name       string
	Qty        int
	PriceCents int64 // unit price in cents
}

// Render writes items to w as an aligned table followed by a total line:
//
//	ITEM    QTY  PRICE  AMOUNT
//	------  ---  -----  ------
//	Coffee    2   3.50    7.00
//	Bagel     1   2.25    2.25
//	------  ---  -----  ------
//	TOTAL                 9.25
//
// The rules:
//   - Columns are ITEM (left-aligned) and QTY, PRICE, AMOUNT
//     (right-aligned), separated by two spaces.
//   - Each column is exactly as wide as its widest cell, header and TOTAL
//     line included, measured in runes (so "Crème" is 5 wide).
//   - Money is printed in dollars with exactly two decimals (1205 cents is
//     "12.05"). AMOUNT is Qty*PriceCents; TOTAL is the sum of the amounts.
//   - Every line ends in "\n" and has no trailing spaces.
func Render(w io.Writer, items []Item) error {
	header := []string{"ITEM", "QTY", "PRICE", "AMOUNT"}
	rows := make([][]string, 0, len(items))
	var total int64
	for _, it := range items {
		amount := int64(it.Qty) * it.PriceCents
		total += it.PriceCents
		rows = append(rows, []string{it.Name, strconv.Itoa(it.Qty), money(it.PriceCents), money(amount)})
	}
	footer := []string{"TOTAL", "", "", money(total)}

	widths := make([]int, len(header))
	for _, row := range append([][]string{header, footer}, rows...) {
		for i, cell := range row {
			widths[i] = max(widths[i], len(cell))
		}
	}
	rule := make([]string, len(widths))
	for i, w := range widths {
		rule[i] = strings.Repeat("-", w)
	}

	var b strings.Builder
	writeRow(&b, widths, header)
	writeRow(&b, widths, rule)
	for _, row := range rows {
		writeRow(&b, widths, row)
	}
	writeRow(&b, widths, rule)
	writeRow(&b, widths, footer)

	_, err := io.WriteString(w, b.String())
	return err
}

// writeRow appends one padded, trimmed line to b. Column 0 is left-aligned,
// the rest right-aligned. fmt's width counts runes, not bytes.
func writeRow(b *strings.Builder, widths []int, cells []string) {
	var line strings.Builder
	for i, cell := range cells {
		if i > 0 {
			line.WriteString("  ")
		}
		if i == 0 {
			fmt.Fprintf(&line, "%-*s", widths[i], cell)
		} else {
			fmt.Fprintf(&line, "%*s", widths[i], cell)
		}
	}
	b.WriteString(strings.TrimRight(line.String(), " "))
	b.WriteByte('\n')
}

// money formats cents as dollars with two decimals. cents must be >= 0.
func money(cents int64) string {
	return fmt.Sprintf("%d.%d", cents/100, cents%100)
}
