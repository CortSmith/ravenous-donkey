// Package wordfreq finds the most frequent words in a text.
package wordfreq

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
)

// WordCount is a word together with the number of times it occurs.
type WordCount struct {
	Word  string
	Count int
}

// Words splits text into lowercase words. A word is a maximal run of Unicode
// letters and digits; every other rune (space, punctuation, symbols) separates
// words.
func Words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), isSeparator)
}

func isSeparator(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// TopN returns the n most frequent words in text, as defined by Words.
//
// Results are ordered by descending count; words with equal counts are
// ordered alphabetically (byte-wise), so the output is fully deterministic.
// If text has fewer than n distinct words, all of them are returned. TopN
// returns nil when n <= 0.
func TopN(text string, n int) []WordCount {
	if n <= 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, w := range Words(text) {
		counts[w]++ // the zero value of a missing key is 0, so no check needed
	}

	all := make([]WordCount, 0, len(counts))
	for w, c := range counts { // map order is random; we sort below
		all = append(all, WordCount{Word: w, Count: c})
	}
	slices.SortFunc(all, func(a, b WordCount) int {
		return cmp.Or(
			cmp.Compare(b.Count, a.Count), // higher count first
			strings.Compare(a.Word, b.Word),
		)
	})
	return all[:min(n, len(all))]
}
