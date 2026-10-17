package semver

import (
	"errors"
	"fmt"
	"strings"
)

// ParseFast accepts exactly the inputs Parse accepts and returns the same
// Version. It walks the string once instead of calling strings.Split, so a
// successful parse allocates nothing (Prerelease and Build are substrings
// of s, as they are in Parse). Error messages may differ from Parse's, but
// every error still wraps ErrInvalid.
func ParseFast(s string) (Version, error) {
	var v Version
	rest := strings.TrimPrefix(s, "v")

	rest, build, hasBuild := strings.Cut(rest, "+")
	if hasBuild {
		if err := checkIdentsFast(build, false); err != nil {
			return Version{}, fmt.Errorf("%w %q: build: %v", ErrInvalid, s, err)
		}
		v.Build = build
	}
	rest, pre, hasPre := strings.Cut(rest, "-")
	if hasPre {
		if err := checkIdentsFast(pre, true); err != nil {
			return Version{}, fmt.Errorf("%w %q: prerelease: %v", ErrInvalid, s, err)
		}
		v.Prerelease = pre
	}

	nums := [3]*int{&v.Major, &v.Minor, &v.Patch}
	for i, n := range nums {
		p := rest
		if i < 2 {
			j := strings.IndexByte(rest, '.')
			if j < 0 {
				return Version{}, fmt.Errorf("%w %q: want MAJOR.MINOR.PATCH", ErrInvalid, s)
			}
			p, rest = rest[:j], rest[j+1:]
		} else if strings.IndexByte(p, '.') >= 0 {
			return Version{}, fmt.Errorf("%w %q: want MAJOR.MINOR.PATCH", ErrInvalid, s)
		}
		x, err := parseNumber(p)
		if err != nil {
			return Version{}, fmt.Errorf("%w %q: %v", ErrInvalid, s, err)
		}
		*n = x
	}
	return v, nil
}

var errEmptyIdent = errors.New("empty identifier")

// checkIdentsFast is checkIdents without strings.Split: it validates each
// identifier in place as it reaches the '.' (or the end) that closes it.
func checkIdentsFast(s string, numericNoZero bool) error {
	start, numeric := 0, true
	for i := 0; i <= len(s); i++ {
		if i < len(s) && s[i] != '.' {
			c := s[i]
			if !isIdentChar(c) {
				return fmt.Errorf("invalid character %q in %q", c, s)
			}
			numeric = numeric && c >= '0' && c <= '9'
			continue
		}
		if i == start {
			return errEmptyIdent
		}
		if numericNoZero && numeric && i-start > 1 && s[start] == '0' {
			return fmt.Errorf("%q has a leading zero", s[start:i])
		}
		start, numeric = i+1, true
	}
	return nil
}
