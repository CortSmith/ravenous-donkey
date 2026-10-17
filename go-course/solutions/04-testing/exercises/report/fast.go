package report

import (
	"io"
	"strconv"
	"unicode/utf8"
)

// RenderFast writes the same bytes as Render. Instead of formatting every
// cell into its own string and padding it with fmt, it measures column
// widths arithmetically, then appends everything into one byte slice and
// writes it with a single Write call.
func RenderFast(w io.Writer, items []Item) error {
	header := [4]string{"ITEM", "QTY", "PRICE", "AMOUNT"}
	var widths [4]int
	for i, h := range header {
		widths[i] = len(h)
	}
	widths[0] = max(widths[0], len("TOTAL"))
	var total int64
	for _, it := range items {
		amount := int64(it.Qty) * it.PriceCents
		total += amount
		widths[0] = max(widths[0], utf8.RuneCountInString(it.Name))
		widths[1] = max(widths[1], digits(int64(it.Qty)))
		widths[2] = max(widths[2], moneyLen(it.PriceCents))
		widths[3] = max(widths[3], moneyLen(amount))
	}
	widths[3] = max(widths[3], moneyLen(total))

	lineLen := widths[0] + widths[1] + widths[2] + widths[3] + 3*2 + 1
	buf := make([]byte, 0, (len(items)+5)*lineLen+len(items)*8) // names may be multi-byte

	buf = padRight(buf, header[0], widths[0])
	for i := 1; i < 4; i++ {
		buf = append(buf, ' ', ' ')
		buf = padLeft(buf, len(header[i]), widths[i])
		buf = append(buf, header[i]...)
	}
	buf = append(buf, '\n')
	buf = appendRule(buf, widths)
	for _, it := range items {
		buf = padRight(buf, it.Name, widths[0])
		buf = append(buf, ' ', ' ')
		buf = padLeft(buf, digits(int64(it.Qty)), widths[1])
		buf = strconv.AppendInt(buf, int64(it.Qty), 10)
		buf = append(buf, ' ', ' ')
		buf = padLeft(buf, moneyLen(it.PriceCents), widths[2])
		buf = appendMoney(buf, it.PriceCents)
		buf = append(buf, ' ', ' ')
		amount := int64(it.Qty) * it.PriceCents
		buf = padLeft(buf, moneyLen(amount), widths[3])
		buf = appendMoney(buf, amount)
		buf = append(buf, '\n')
	}
	buf = appendRule(buf, widths)
	buf = append(buf, "TOTAL"...)
	buf = padLeft(buf, len("TOTAL"), widths[0]+2+widths[1]+2+widths[2]+2+widths[3]-moneyLen(total))
	buf = appendMoney(buf, total)
	buf = append(buf, '\n')

	_, err := w.Write(buf)
	return err
}

// padRight appends s followed by spaces up to width runes. The first
// column is never right-trimmed: at least the QTY column follows it.
func padRight(buf []byte, s string, width int) []byte {
	buf = append(buf, s...)
	for range width - utf8.RuneCountInString(s) {
		buf = append(buf, ' ')
	}
	return buf
}

// padLeft appends the spaces needed to right-align a cell of length n.
func padLeft(buf []byte, n, width int) []byte {
	for range width - n {
		buf = append(buf, ' ')
	}
	return buf
}

func appendRule(buf []byte, widths [4]int) []byte {
	for i, w := range widths {
		if i > 0 {
			buf = append(buf, ' ', ' ')
		}
		for range w {
			buf = append(buf, '-')
		}
	}
	return append(buf, '\n')
}

// appendMoney appends cents formatted like money (cents >= 0).
func appendMoney(buf []byte, cents int64) []byte {
	buf = strconv.AppendInt(buf, cents/100, 10)
	c := cents % 100
	return append(buf, '.', byte('0'+c/10), byte('0'+c%10))
}

func moneyLen(cents int64) int { return digits(cents/100) + 3 }

// digits returns the number of decimal digits in n (n >= 0).
func digits(n int64) int {
	d := 1
	for n >= 10 {
		n /= 10
		d++
	}
	return d
}
