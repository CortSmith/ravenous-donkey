package anagram

import "unicode/utf8"

// IsAnagramFast behaves exactly like IsAnagram but does not allocate when
// both inputs are pure ASCII, which is the overwhelmingly common case.
//
// Instead of a map[rune]int it counts into a fixed [36]int array on the
// stack (26 letters + 10 digits) indexed by a cheap byte computation. As
// soon as a non-ASCII byte is seen it falls back to the general version, so
// the Unicode semantics are unchanged.
func IsAnagramFast(a, b string) bool {
	var counts [36]int
	if !countASCII(&counts, a, 1) || !countASCII(&counts, b, -1) {
		return IsAnagram(a, b)
	}
	return counts == [36]int{} // arrays are comparable with ==
}

// countASCII adds delta to counts for each letter/digit in s. It returns
// false, leaving counts partially updated, if s contains a non-ASCII byte.
func countASCII(counts *[36]int, s string, delta int) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= utf8.RuneSelf:
			return false
		case 'a' <= c && c <= 'z':
			counts[c-'a'] += delta
		case 'A' <= c && c <= 'Z':
			counts[c-'A'] += delta
		case '0' <= c && c <= '9':
			counts[26+c-'0'] += delta
		}
	}
	return true
}
