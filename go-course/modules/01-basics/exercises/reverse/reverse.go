// Package reverse reverses strings in a UTF-8-aware way.
package reverse

// Reverse returns s with its runes (Unicode code points) in reverse order.
//
// Each invalid UTF-8 byte in s becomes utf8.RuneError (U+FFFD) in the result,
// exactly as a conversion to []rune would do. Combining marks and emoji
// modifiers are reversed independently of the character they modify, because
// a rune is a code point, not a user-perceived character.
func Reverse(s string) string {
	// TODO: reverse by rune, not by byte. Reversing the bytes of "世界"
	// produces invalid UTF-8. Hint: []rune(s) decodes, string(runes) encodes.
	return ""
}

// ReverseWords returns the whitespace-separated words of s in reverse order,
// joined by single spaces. Leading, trailing and repeated whitespace is
// dropped. The words themselves are not reversed.
func ReverseWords(s string) string {
	// TODO: look at strings.Fields, slices.Reverse and strings.Join.
	return ""
}
