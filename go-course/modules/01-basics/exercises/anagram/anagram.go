// Package anagram detects anagrams in Unicode text.
package anagram

// IsAnagram reports whether a and b are anagrams of each other: whether they
// contain the same letters and digits the same number of times, ignoring
// letter case and ignoring every rune that is not a letter or digit (spaces,
// punctuation, symbols, combining marks).
//
// Case is compared with unicode.ToLower, so "Ä" and "ä" match. No Unicode
// normalization is performed: a precomposed "é" (U+00E9) does not match "e"
// followed by a combining accent (U+0301). Two strings with no letters or
// digits at all are anagrams of each other.
func IsAnagram(a, b string) bool {
	// TODO: range over each string (which yields runes, not bytes) and keep
	// per-rune counts. Comparing len(a) and len(b) up front is a bug: why?
	return false
}
