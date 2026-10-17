package semver

import "math"

// ParseFast is a single-pass variant of Parse with identical results. It
// walks s once, accumulating each component's digits as it goes, and checks
// overflow by hand instead of calling strconv.Atoi. It is harder to read and
// to get right (leading zeros, overflow, component count), which is why Parse
// is the version to ship unless a profile says this function is hot.
func ParseFast(s string) (Version, error) {
	i := 0
	if len(s) > 0 && s[0] == 'v' {
		i = 1
	}
	var nums [3]int
	for k := range nums {
		start := i
		n := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			d := int(s[i] - '0')
			if n > (math.MaxInt-d)/10 {
				return Version{}, invalid(s)
			}
			n = n*10 + d
			i++
		}
		digits := i - start
		if digits == 0 || (digits > 1 && s[start] == '0') {
			return Version{}, invalid(s)
		}
		nums[k] = n
		if k < 2 {
			if i >= len(s) || s[i] != '.' {
				return Version{}, invalid(s)
			}
			i++
		}
	}
	if i != len(s) {
		return Version{}, invalid(s)
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}
