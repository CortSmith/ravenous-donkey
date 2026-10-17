package reverse

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func TestReverseFastMatchesReverse(t *testing.T) {
	inputs := []string{
		"", "a", "Hello, 世界", "a😀b🎉", "éx", "a\xffb", "\xe4\xb8x",
		"\xf0\x9f\x98", "\xed\xa0\x80", "� literal", "\xc0\xaf",
	}
	// Random byte strings exercise the invalid-UTF-8 paths thoroughly.
	rng := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		b := make([]byte, rng.IntN(12))
		for i := range b {
			b[i] = byte(rng.IntN(256))
		}
		inputs = append(inputs, string(b))
	}
	for _, in := range inputs {
		if got, want := ReverseFast(in), Reverse(in); got != want {
			t.Fatalf("ReverseFast(%q) = %q, want %q", in, got, want)
		}
	}
}

var benchInput = strings.Repeat("Hello, 世界! Gophers 🚀 ", 50)

func BenchmarkReverse(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		Reverse(benchInput)
	}
}

func BenchmarkReverseFast(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		ReverseFast(benchInput)
	}
}
