package reverse

import (
	"testing"
	"unicode/utf8"
)

func TestReverse(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"single byte", "a", "a"},
		{"ascii", "Hello, World", "dlroW ,olleH"},
		{"palindrome", "racecar", "racecar"},
		{"two-byte runes", "héllo", "olléh"},
		{"three-byte runes", "Hello, 世界", "界世 ,olleH"},
		{"four-byte runes", "a😀b🎉", "🎉b😀a"},
		{"cyrillic", "Привет", "тевирП"},
		// A rune is a code point, not a "character": the combining acute
		// accent (U+0301) moves to the other side of its base letter.
		{"combining mark", "éx", "x́e"},
		// Invalid bytes become U+FFFD, just like []rune(s).
		{"invalid byte", "a\xffb", "b�a"},
		{"truncated sequence", "\xe4\xb8x", "x��"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Reverse(tt.in); got != tt.want {
				t.Errorf("Reverse(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestReverseProperties(t *testing.T) {
	inputs := []string{
		"", "x", "go", "Gopher", "naïve café", "日本語テキスト", "👍🏽 ok",
		"mixed ascii and 中文 and émoji 🚀", "\xff\xfe", "ok\xc3",
	}
	for _, in := range inputs {
		got := Reverse(in)
		if !utf8.ValidString(got) {
			t.Errorf("Reverse(%q) = %q, which is not valid UTF-8", in, got)
		}
		if gotN, wantN := utf8.RuneCountInString(got), utf8.RuneCountInString(in); gotN != wantN {
			t.Errorf("Reverse(%q) has %d runes, want %d (same as input)", in, gotN, wantN)
		}
		if utf8.ValidString(in) {
			if back := Reverse(got); back != in {
				t.Errorf("Reverse(Reverse(%q)) = %q, want the original string back", in, back)
			}
		}
	}
}

func TestReverseWords(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"one", "one"},
		{"hello world", "world hello"},
		{"the quick brown fox", "fox brown quick the"},
		{"  padded\tand \n spaced  ", "spaced and padded"},
		{"héllo 世界", "世界 héllo"},
	}
	for _, tt := range tests {
		if got := ReverseWords(tt.in); got != tt.want {
			t.Errorf("ReverseWords(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
