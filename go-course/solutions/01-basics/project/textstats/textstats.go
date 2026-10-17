// Package textstats computes simple, deterministic statistics about text:
// word counts, vocabulary size, the most frequent words, the longest word and
// the average word length.
package textstats

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// WordCount is a word together with the number of times it occurs.
type WordCount struct {
	Word  string
	Count int
}

// Options controls Analyze.
type Options struct {
	// TopN is how many of the most frequent words to report. Values <= 0
	// report none.
	TopN int
	// Stopwords lists words to leave out of Report.Top. Matching is
	// case-insensitive. Stopwords still count towards every other statistic.
	Stopwords []string
}

// Report holds the result of Analyze.
type Report struct {
	Words         int         // total number of words
	Unique        int         // number of distinct words
	Top           []WordCount // most frequent non-stopwords, at most Options.TopN
	Longest       string      // longest word in runes; ties go to the alphabetically first
	AverageLength float64     // mean word length in runes; 0 if there are no words
}

// DefaultStopwords is a small list of very common English words.
// Callers must treat it as read-only.
var DefaultStopwords = []string{
	"a", "an", "and", "are", "as", "at", "be", "but", "by", "for", "from",
	"had", "has", "have", "he", "her", "his", "i", "in", "is", "it", "its",
	"not", "of", "on", "or", "she", "that", "the", "their", "they", "this",
	"to", "was", "we", "were", "will", "with", "you",
}

// Tokenize splits text into lowercase words.
//
// A word is a maximal run of Unicode letters and digits. An apostrophe
// (U+0027 ' or U+2019 ’) is part of a word only when it sits directly between
// two letters or digits, as in "don't" or "rock'n'roll"; it is always
// normalized to U+0027 in the output. All other runes separate words.
func Tokenize(text string) []string {
	var (
		words []string
		cur   strings.Builder
	)
	runes := []rune(text)
	for i, r := range runes {
		switch {
		case isWordRune(r):
			cur.WriteRune(unicode.ToLower(r))
		case isApostrophe(r) && cur.Len() > 0 && i+1 < len(runes) && isWordRune(runes[i+1]):
			cur.WriteByte('\'')
		case cur.Len() > 0:
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isApostrophe(r rune) bool {
	return r == '\'' || r == '’'
}

// Analyze computes a Report for text. Its output depends only on its inputs:
// it never depends on map iteration order. Analyze does not modify
// opts.Stopwords.
func Analyze(text string, opts Options) Report {
	words := Tokenize(text)
	var r Report
	r.Words = len(words)
	if len(words) == 0 {
		return r
	}

	counts := make(map[string]int)
	totalRunes := 0
	for _, w := range words {
		counts[w]++
		n := utf8.RuneCountInString(w)
		totalRunes += n
		longest := utf8.RuneCountInString(r.Longest)
		if n > longest || (n == longest && w < r.Longest) {
			r.Longest = w
		}
	}
	r.Unique = len(counts)
	r.AverageLength = float64(totalRunes) / float64(len(words))
	r.Top = topWords(counts, stopSet(opts.Stopwords), opts.TopN)
	return r
}

// stopSet builds a lowercase lookup set from a stopword list.
func stopSet(words []string) map[string]struct{} {
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[strings.ToLower(w)] = struct{}{}
	}
	return set
}

// topWords returns the n most frequent words in counts that are not in stop,
// ordered by descending count and then alphabetically.
func topWords(counts map[string]int, stop map[string]struct{}, n int) []WordCount {
	if n <= 0 {
		return nil
	}
	var all []WordCount
	for w, c := range counts {
		if _, ok := stop[w]; ok {
			continue
		}
		all = append(all, WordCount{Word: w, Count: c})
	}
	slices.SortFunc(all, func(a, b WordCount) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), strings.Compare(a.Word, b.Word))
	})
	// Clone so the result doesn't pin the (possibly huge) sorted array.
	return slices.Clone(all[:min(n, len(all))])
}

// String formats the report as human-readable text, for example:
//
//	words: 9
//	unique: 8
//	longest: jumps
//	average length: 3.89
//	top words:
//	  1. fox (2)
//	  2. brown (1)
//
// When Top is empty the last line reads "top words: none".
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "words: %d\n", r.Words)
	fmt.Fprintf(&b, "unique: %d\n", r.Unique)
	fmt.Fprintf(&b, "longest: %s\n", r.Longest)
	fmt.Fprintf(&b, "average length: %.2f\n", r.AverageLength)
	if len(r.Top) == 0 {
		b.WriteString("top words: none\n")
		return b.String()
	}
	b.WriteString("top words:\n")
	for i, wc := range r.Top {
		fmt.Fprintf(&b, "  %d. %s (%d)\n", i+1, wc.Word, wc.Count)
	}
	return b.String()
}
