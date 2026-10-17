// Package semver parses simple MAJOR.MINOR.PATCH version strings.
package semver

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
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
// This version is CORRECT but SLOW: it compiles a regular expression on every
// call, and FindStringSubmatch allocates a slice of submatches each time.
//
// TODO: keep the behaviour, but make Parse allocation-free for valid input.
// (Hoisting the regexp to a package variable is a good first step. Is it
// enough? Measure!)
func Parse(s string) (Version, error) {
	re := regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
	}
	var nums [3]int
	for i, part := range m[1:] {
		n, err := strconv.Atoi(part)
		if err != nil { // overflow
			return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}
