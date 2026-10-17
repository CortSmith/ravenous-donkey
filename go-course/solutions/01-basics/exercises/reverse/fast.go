package reverse

import (
	"strings"
	"unicode/utf8"
)

// ReverseFast behaves exactly like Reverse but performs a single allocation.
//
// Instead of decoding the whole string into a []rune (4 bytes per rune) and
// re-encoding it, it walks s backwards one encoded rune at a time and copies
// the original bytes straight into a pre-sized strings.Builder. Only invalid
// bytes need re-encoding (as U+FFFD, which is 3 bytes long, so the Grow is a
// hint rather than an exact size for invalid input).
func ReverseFast(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := len(s); i > 0; {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError) // invalid byte: match []rune semantics
		} else {
			b.WriteString(s[i-size : i])
		}
		i -= size
	}
	return b.String()
}
