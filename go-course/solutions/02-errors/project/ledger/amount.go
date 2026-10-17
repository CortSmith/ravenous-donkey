package ledger

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseAmount converts a decimal amount such as "12", "12.5" or "12.50" into
// cents. It accepts one or more digits, optionally followed by '.' and one or
// two digits. Signs, exponents, spaces, a leading or trailing '.' and values
// that overflow int64 cents are rejected with an error wrapping
// ErrInvalidAmount, e.g. `invalid amount "1.234"`. Zero is accepted here;
// ledger operations decide whether zero is allowed.
func ParseAmount(s string) (int64, error) {
	bad := func() (int64, error) {
		return 0, fmt.Errorf("%w %q", ErrInvalidAmount, s)
	}

	whole, frac, hasDot := strings.Cut(s, ".")
	if !allDigits(whole) || (hasDot && (len(frac) < 1 || len(frac) > 2 || !allDigits(frac))) {
		return bad()
	}

	units, err := strconv.ParseInt(whole, 10, 64)
	if err != nil { // only possible error after allDigits: value out of range
		return bad()
	}
	var cents int64
	if hasDot {
		cents, _ = strconv.ParseInt(frac, 10, 64) // 1-2 digits: cannot fail
		if len(frac) == 1 {
			cents *= 10 // "12.5" means 50 cents
		}
	}
	if units > (math.MaxInt64-cents)/100 {
		return bad()
	}
	return units*100 + cents, nil
}

// allDigits reports whether s is non-empty and consists of ASCII digits.
func allDigits(s string) bool {
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
	sign := ""
	u := uint64(cents)
	if cents < 0 {
		sign = "-"
		u = -u // two's complement negation; correct even for MinInt64
	}
	return fmt.Sprintf("%s%d.%02d", sign, u/100, u%100)
}
