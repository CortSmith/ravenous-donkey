// Package semver parses simple MAJOR.MINOR.PATCH version strings.
package semver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalid is wrapped by every error Parse returns.
var ErrInvalid = errors.New("invalid version")

// Version is a parsed MAJOR.MINOR.PATCH version.
type Version struct {
	Major, Minor, Patch int
}

// Parse parses s as "MAJOR.MINOR.PATCH" with an optional leading "v".
// Each component is a non-negative decimal integer without a sign or leading
// zeros ("0" itself is allowed) that fits in an int. Pre-release and build
// suffixes ("-rc1", "+meta") are not supported. On failure the returned error
// wraps ErrInvalid.
//
// Parse does not allocate for valid input: strings.Cut returns substrings of s
// (no copies), and strconv.Atoi does not allocate on success.
func Parse(s string) (Version, error) {
	rest := strings.TrimPrefix(s, "v")
	major, rest, ok1 := strings.Cut(rest, ".")
	minor, patch, ok2 := strings.Cut(rest, ".")
	if !ok1 || !ok2 {
		return Version{}, invalid(s)
	}
	var v Version
	var ok bool
	if v.Major, ok = parseComponent(major); !ok {
		return Version{}, invalid(s)
	}
	if v.Minor, ok = parseComponent(minor); !ok {
		return Version{}, invalid(s)
	}
	if v.Patch, ok = parseComponent(patch); !ok {
		return Version{}, invalid(s)
	}
	return v, nil
}

// parseComponent parses one version component. It reports false for an empty
// string, a leading zero, any non-digit (including a third '.'), or overflow.
func parseComponent(s string) (int, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false // also rejects "+1", which Atoi would accept
		}
	}
	n, err := strconv.Atoi(s) // only fails on overflow now
	return n, err == nil
}

// invalid builds the error for s. It is only reached on bad input, so its
// allocations do not matter for the fast path.
func invalid(s string) error {
	return fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
}
