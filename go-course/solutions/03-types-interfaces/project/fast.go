package inventory

import (
	"io"
	"slices"
	"strconv"
)

// WriteReportFast produces exactly the same bytes as WriteReport but builds
// the whole report in one []byte with strconv.Append* and manual padding,
// then issues a single Write. Compared with WriteReport it avoids fmt's
// reflection-based formatting, the per-row FormatCents/fmt.Sprint strings
// and one Write call per line.
//
// Trade-off: the column layout now lives in code (widths passed to padLeft /
// padRight) instead of one readable format string, and the whole report is
// held in memory before writing.
func (inv *Inventory) WriteReportFast(w io.Writer, key SortKey) error {
	ps, err := inv.Sorted(key)
	if err != nil {
		return err
	}
	buf := make([]byte, 0, 64*(len(ps)+3))
	buf = appendRow(buf, "SKU", "NAME", []byte("QTY"), []byte("PRICE"), []byte("VALUE"))

	var totalQty int
	var totalValue int64
	low := make([]string, 0, 8)
	var qty, price, value [24]byte // scratch space for numbers, on the stack
	for _, p := range ps {
		buf = appendRow(buf, p.SKU, p.Name,
			strconv.AppendInt(qty[:0], int64(p.Quantity), 10),
			appendCents(price[:0], p.PriceCents),
			appendCents(value[:0], p.Value()))
		totalQty += p.Quantity
		totalValue += p.Value()
		if p.LowStock() {
			low = append(low, p.SKU)
		}
	}
	buf = appendRow(buf, "TOTAL", "",
		strconv.AppendInt(qty[:0], int64(totalQty), 10), nil,
		appendCents(value[:0], totalValue))

	buf = append(buf, "low stock: "...)
	if len(low) == 0 {
		buf = append(buf, "none"...)
	}
	slices.Sort(low)
	for i, s := range low {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = append(buf, s...)
	}
	buf = append(buf, '\n')
	_, err = w.Write(buf)
	return err
}

// appendRow appends one line laid out like "%-8s %-20s %5s %10s %12s\n".
func appendRow(buf []byte, sku, name string, qty, price, value []byte) []byte {
	buf = padRight(buf, sku, 8)
	buf = append(buf, ' ')
	buf = padRight(buf, name, 20)
	buf = append(buf, ' ')
	buf = padLeft(buf, qty, 5)
	buf = append(buf, ' ')
	buf = padLeft(buf, price, 10)
	buf = append(buf, ' ')
	buf = padLeft(buf, value, 12)
	return append(buf, '\n')
}

// padRight mimics %-Ns. Like fmt, width counts runes, not bytes.
func padRight(buf []byte, s string, width int) []byte {
	buf = append(buf, s...)
	for n := runeCount(s); n < width; n++ {
		buf = append(buf, ' ')
	}
	return buf
}

// padLeft mimics %Ns for ASCII input (numbers and money only).
func padLeft(buf, s []byte, width int) []byte {
	for n := len(s); n < width; n++ {
		buf = append(buf, ' ')
	}
	return append(buf, s...)
}

func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// appendCents is the append-style version of FormatCents.
func appendCents(buf []byte, cents int64) []byte {
	u := uint64(cents)
	if cents < 0 {
		buf = append(buf, '-')
		u = -u
	}
	buf = append(buf, '$')
	buf = strconv.AppendUint(buf, u/100, 10)
	frac := u % 100
	return append(buf, '.', byte('0'+frac/10), byte('0'+frac%10))
}
