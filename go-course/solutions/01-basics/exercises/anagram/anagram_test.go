package anagram

import "testing"

func TestIsAnagram(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"both empty", "", "", true},
		{"identical", "listen", "listen", true},
		{"simple", "listen", "silent", true},
		{"mixed case", "Listen", "SILENT", true},
		{"spaces ignored", "dormitory", "dirty room", true},
		{"punctuation ignored", "Dormitory!", "Dirty, room.", true},
		{"phrase", "Tom Marvolo Riddle", "I am Lord Voldemort", true},
		{"astronomer", "Astronomer", "Moon starer", true},
		{"digits", "a1b2", "2b1a", true},
		{"only punctuation both sides", "?!", "...", true},
		{"different letters", "hello", "world", false},
		{"same letters, different counts", "aab", "abb", false},
		{"extra letter", "abc", "abcd", false},
		{"one side empty", "", "a", false},
		{"letters vs nothing", "a!", "!", false},
		{"digits differ", "123", "124", false},
		{"accented letters", "Ärger", "regär", true},
		{"accent matters", "resume", "résumé", false},
		{"greek", "ΑΒΓ", "γβα", true},
		{"cyrillic", "Апельсин", "спаниель", true},
		{"cjk", "日本語", "語本日", true},
		{"emoji ignored", "go🚀", "🎉og", true},
		{"no normalization", "é", "é", false},
		{"multibyte count mismatch", "ééa", "éaa", false},
		{"invalid utf8 ignored", "ab\xff", "ba", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAnagram(tt.a, tt.b); got != tt.want {
				t.Errorf("IsAnagram(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
			if got := IsAnagram(tt.b, tt.a); got != tt.want {
				t.Errorf("IsAnagram(%q, %q) = %v, want %v (anagram is symmetric)", tt.b, tt.a, got, tt.want)
			}
		})
	}
}
