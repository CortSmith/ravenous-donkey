// Package reverse reverses strings in a UTF-8-aware way.
package reverse

import (
	"slices"
	"strings"
)

// Reverse returns s with its runes (Unicode code points) in reverse order.
//
// Each invalid UTF-8 byte in s becomes utf8.RuneError (U+FFFD) in the result,
// exactly as a conversion to []rune would do. Combining marks and emoji
// modifiers are reversed independently of the character they modify, because
// a rune is a code point, not a user-perceived character.
func Reverse(s string) string {
	r := []rune(s) // decodes UTF-8; one element per code point
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r) // re-encodes as UTF-8
}

// ReverseWords returns the whitespace-separated words of s in reverse order,
// joined by single spaces. Leading, trailing and repeated whitespace is
// dropped. The words themselves are not reversed.
func ReverseWords(s string) string {
	words := strings.Fields(s)
	slices.Reverse(words)
	return strings.Join(words, " ")
}
