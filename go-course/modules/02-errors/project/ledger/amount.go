package ledger

import (
	"fmt"
)

// ParseAmount converts a decimal amount such as "12", "12.5" or "12.50" into
// cents. It accepts one or more digits, optionally followed by '.' and one or
// two digits. Signs, exponents, spaces, a leading or trailing '.' and values
// that overflow int64 cents are rejected with an error wrapping
// ErrInvalidAmount, e.g. `invalid amount "1.234"`. Zero is accepted here;
// ledger operations decide whether zero is allowed.
func ParseAmount(s string) (int64, error) {
	// TODO: Implement ParseAmount.
	// - Use strings.Cut to split on '.'.
	// - Validate format: all digits in the whole part, 1-2 digits in the fraction.
	// - Convert to int64 cents, multiplying units by 100.
	// - Pad the fraction if needed (e.g. "12.5" = 50 cents).
	// - Check for overflow.
	// - Return fmt.Errorf("%w %q", ErrInvalidAmount, s) on error.
	return 0, fmt.Errorf("%w %q", ErrInvalidAmount, s)
}

// allDigits reports whether s is non-empty and consists of ASCII digits.
func allDigits(s string) bool {
	// TODO: Implement allDigits.
	// Return true if s is non-empty and all bytes are ASCII digits.
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// FormatAmount formats cents as a decimal with exactly two fraction digits:
// 1234 -> "12.34", 5 -> "0.05", -150 -> "-1.50".
func FormatAmount(cents int64) string {
	// TODO: Implement FormatAmount.
	// Handle negative amounts correctly.
	// Use fmt.Sprintf to format with exactly 2 decimal places.
	sign := ""
	u := uint64(cents)
	if cents < 0 {
		sign = "-"
		u = -u // two's complement negation; correct even for MinInt64
	}
	return fmt.Sprintf("%s%d.%02d", sign, u/100, u%100)
}
