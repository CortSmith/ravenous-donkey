// Package textstats computes simple, deterministic statistics about text:
// word counts, vocabulary size, the most frequent words, the longest word and
// the average word length.
package textstats

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
	// TODO: walk the runes. strings.FieldsFunc can't see the next rune, so
	// it can't implement the apostrophe rule on its own. A strings.Builder
	// for the current word works well.
	return nil
}

// Analyze computes a Report for text. Its output depends only on its inputs:
// it never depends on map iteration order. Analyze does not modify
// opts.Stopwords.
func Analyze(text string, opts Options) Report {
	// TODO: tokenize, count with a map, build a stopword set, pick the top
	// words with a deterministic sort, and measure lengths in runes
	// (utf8.RuneCountInString), not bytes.
	return Report{}
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
	// TODO: build the output with a strings.Builder (fmt.Fprintf works on
	// it because *strings.Builder is an io.Writer).
	return ""
}
