// Package wordfreq finds the most frequent words in a text.
package wordfreq

// WordCount is a word together with the number of times it occurs.
type WordCount struct {
	Word  string
	Count int
}

// Words splits text into lowercase words. A word is a maximal run of Unicode
// letters and digits; every other rune (space, punctuation, symbols) separates
// words.
func Words(text string) []string {
	// TODO: strings.FieldsFunc with a predicate built from unicode.IsLetter
	// and unicode.IsDigit does most of the work.
	return nil
}

// TopN returns the n most frequent words in text, as defined by Words.
//
// Results are ordered by descending count; words with equal counts are
// ordered alphabetically (byte-wise), so the output is fully deterministic.
// If text has fewer than n distinct words, all of them are returned. TopN
// returns nil when n <= 0.
func TopN(text string, n int) []WordCount {
	// TODO: count into a map, copy into a slice, sort with slices.SortFunc
	// (cmp.Compare and cmp.Or help with the two-key order), then truncate.
	// Remember: map iteration order is random.
	return nil
}
