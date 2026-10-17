package wordfreq

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestTopNFastMatchesTopN(t *testing.T) {
	texts := []string{
		"", "!!", "The quick brown fox jumps over the lazy dog. The dog sleeps; the fox runs!",
		"pear apple fig apple pear fig banana", "Ça va? ça VA! ÇA", "日本 語 日本", "x\xffy x",
		"Straße STRASSE strasse", "2024 2025 2024",
	}
	rng := rand.New(rand.NewPCG(3, 4))
	vocab := []string{"go", "Go", "gopher", "élan", "ÉLAN", "日本", "42", "a", "b", "z"}
	seps := []string{" ", ", ", "\n", "--", "!"}
	for range 200 {
		var sb strings.Builder
		for range rng.IntN(60) {
			sb.WriteString(vocab[rng.IntN(len(vocab))])
			sb.WriteString(seps[rng.IntN(len(seps))])
		}
		texts = append(texts, sb.String())
	}
	for _, text := range texts {
		for _, n := range []int{-1, 0, 1, 2, 3, 5, 100} {
			got, want := TopNFast(text, n), TopN(text, n)
			if len(got) == 0 && len(want) == 0 {
				continue
			}
			if !slices.Equal(got, want) {
				t.Fatalf("TopNFast(%q, %d) = %v, want %v", text, n, got, want)
			}
		}
	}
}

var benchText = func() string {
	rng := rand.New(rand.NewPCG(5, 6))
	var sb strings.Builder
	for range 20000 {
		// Zipf-ish: small word ids are far more common.
		fmt.Fprintf(&sb, "Word%d, ", rng.IntN(1+rng.IntN(2000)))
	}
	return sb.String()
}()

func BenchmarkTopN(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		TopN(benchText, 10)
	}
}

func BenchmarkTopNFast(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		TopNFast(benchText, 10)
	}
}
