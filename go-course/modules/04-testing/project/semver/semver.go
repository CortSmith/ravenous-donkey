// Package semver parses and compares Semantic Versions as defined by
// https://semver.org/spec/v2.0.0.html.
//
// This package has no tests and 4 bugs. See ../README.md.
package semver

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ErrInvalid is wrapped by every error Parse returns.
var ErrInvalid = errors.New("semver: invalid version")

// Version is a parsed semantic version such as 1.4.2-rc.1+build.7.
type Version struct {
	Major, Minor, Patch int

	// Prerelease holds the dot-separated pre-release identifiers without
	// the leading '-' ("rc.1"), or "" for a normal release.
	Prerelease string

	// Build holds the dot-separated build metadata without the leading '+'
	// ("build.7"), or "". Build metadata is ignored by Compare.
	Build string
}

// Parse parses a version of the form MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD],
// optionally preceded by "v".
//
// MAJOR, MINOR and PATCH are non-negative decimal integers without leading
// zeros ("0" is fine, "01" is not). PRERELEASE and BUILD are non-empty,
// dot-separated lists of non-empty identifiers made of [0-9A-Za-z-]; a
// PRERELEASE identifier made only of digits must not have leading zeros.
// Every error wraps ErrInvalid.
func Parse(s string) (Version, error) {
	var v Version
	rest := strings.TrimPrefix(s, "v")

	rest, build, hasBuild := strings.Cut(rest, "+")
	if hasBuild {
		if err := checkIdents(build, false); err != nil {
			return Version{}, fmt.Errorf("%w %q: build: %v", ErrInvalid, s, err)
		}
		v.Build = build
	}
	rest, pre, hasPre := strings.Cut(rest, "-")
	if hasPre {
		if err := checkIdents(pre, true); err != nil {
			return Version{}, fmt.Errorf("%w %q: prerelease: %v", ErrInvalid, s, err)
		}
		v.Prerelease = pre
	}

	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%w %q: want MAJOR.MINOR.PATCH", ErrInvalid, s)
	}
	nums := [3]*int{&v.Major, &v.Minor, &v.Patch}
	for i, p := range parts {
		n, err := parseNumber(p)
		if err != nil {
			return Version{}, fmt.Errorf("%w %q: %v", ErrInvalid, s, err)
		}
		*nums[i] = n
	}
	return v, nil
}

// MustParse is like Parse but panics on error. Use it only for constants
// in tests and package-level variables.
func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// parseNumber parses one MAJOR/MINOR/PATCH component.
func parseNumber(p string) (int, error) {
	if !isNumeric(p) {
		return 0, fmt.Errorf("%q is not a number", p)
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return 0, fmt.Errorf("%q is out of range", p)
	}
	return n, nil
}

// checkIdents validates a dot-separated identifier list. numericNoZero
// enables the pre-release rule against leading zeros in numeric ids.
func checkIdents(s string, numericNoZero bool) error {
	for _, id := range strings.Split(s, ".") {
		for _, c := range []byte(id) {
			if !isIdentChar(c) {
				return fmt.Errorf("invalid character %q in %q", c, id)
			}
		}
		if numericNoZero && isNumeric(id) && id[0] == '0' && len(id) > 1 {
			return fmt.Errorf("%q has a leading zero", id)
		}
	}
	return nil
}

func isIdentChar(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-'
}

// isNumeric reports whether s is made only of ASCII digits.
func isNumeric(s string) bool {
	for _, c := range []byte(s) {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// String returns the canonical form of v, without a "v" prefix.
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// IsPrerelease reports whether v has pre-release identifiers.
func (v Version) IsPrerelease() bool { return v.Prerelease != "" }

// Compare returns -1, 0 or +1 depending on whether a has lower, equal or
// higher precedence than b, following semver §11:
//   - MAJOR, MINOR, PATCH compare numerically, in that order;
//   - a pre-release version is lower than the same version without one
//     (1.0.0-rc.1 < 1.0.0);
//   - pre-release identifiers compare left to right: numeric ones
//     numerically, others in ASCII order, numeric lower than non-numeric,
//     and a shorter list is lower when all its identifiers are equal;
//   - Build metadata is ignored.
//
// Compare's signature fits slices.SortFunc.
func Compare(a, b Version) int {
	if c := cmp.Compare(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Patch, b.Patch); c != 0 {
		return c
	}
	return comparePrerelease(a.Prerelease, b.Prerelease)
}

func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range min(len(as), len(bs)) {
		if c := compareIdent(as[i], bs[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(as), len(bs))
}

func compareIdent(a, b string) int {
	an, bn := isNumeric(a), isNumeric(b)
	switch {
	case an && bn:
		return strings.Compare(a, b)
	case an:
		return -1
	case bn:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// Sort sorts vs in ascending precedence. Versions of equal precedence (that
// differ only in build metadata) keep their relative order.
func Sort(vs []Version) {
	slices.SortStableFunc(vs, Compare)
}
