package textstats

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"only separators", "  -- !? \n", nil},
		{"lowercases", "Hello WORLD", []string{"hello", "world"}},
		{"punctuation splits", "one,two;three.four", []string{"one", "two", "three", "four"}},
		{"digits are word runes", "Go 1.24", []string{"go", "1", "24"}},
		{"inner apostrophe kept", "Don't panic", []string{"don't", "panic"}},
		{"curly apostrophe normalized", "Don’t panic", []string{"don't", "panic"}},
		{"multiple apostrophes", "rock'n'roll", []string{"rock'n'roll"}},
		{"leading apostrophe dropped", "'tis", []string{"tis"}},
		{"trailing apostrophe dropped", "dogs' bones", []string{"dogs", "bones"}},
		{"quoted word", "'hello'", []string{"hello"}},
		{"double apostrophe splits", "a''b", []string{"a", "b"}},
		{"apostrophe alone", " ' ", nil},
		{"unicode letters", "Ça va, Zoë? Ünïcödé!", []string{"ça", "va", "zoë", "ünïcödé"}},
		{"cjk", "日本語 テキスト", []string{"日本語", "テキスト"}},
		{"hyphen splits", "well-known", []string{"well", "known"}},
		{"emoji separates", "go🚀fast", []string{"go", "fast"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Tokenize(tt.in)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Tokenize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

const sample = `The quick brown fox jumps over the lazy dog.
The dog barks; the fox runs away. Isn't the fox quick?`

func TestAnalyzeSample(t *testing.T) {
	got := Analyze(sample, Options{TopN: 3, Stopwords: DefaultStopwords})
	want := Report{
		Words:         20,
		Unique:        12,
		Top:           []WordCount{{"fox", 3}, {"dog", 2}, {"quick", 2}},
		Longest:       "barks", // five 5-rune words; "barks" sorts first
		AverageLength: 3.8,     // 76 runes / 20 words
	}
	assertReport(t, got, want)
}

func TestAnalyze(t *testing.T) {
	tests := []struct {
		name string
		text string
		opts Options
		want Report
	}{
		{
			name: "empty text",
			text: "",
			opts: Options{TopN: 5},
			want: Report{},
		},
		{
			name: "only punctuation",
			text: "... !!! ---",
			opts: Options{TopN: 5, Stopwords: DefaultStopwords},
			want: Report{},
		},
		{
			name: "no stopwords",
			text: "the cat and the hat",
			opts: Options{TopN: 2},
			want: Report{Words: 5, Unique: 4, Top: []WordCount{{"the", 2}, {"and", 1}}, Longest: "and", AverageLength: 3},
		},
		{
			name: "stopwords excluded from top only",
			text: "the cat and the hat",
			opts: Options{TopN: 5, Stopwords: DefaultStopwords},
			want: Report{Words: 5, Unique: 4, Top: []WordCount{{"cat", 1}, {"hat", 1}}, Longest: "and", AverageLength: 3},
		},
		{
			name: "stopwords are case-insensitive",
			text: "Alpha beta ALPHA gamma",
			opts: Options{TopN: 5, Stopwords: []string{"ALPHA", "Gamma"}},
			want: Report{Words: 4, Unique: 3, Top: []WordCount{{"beta", 1}}, Longest: "alpha", AverageLength: 4.75},
		},
		{
			name: "all words are stopwords",
			text: "the a an the",
			opts: Options{TopN: 3, Stopwords: DefaultStopwords},
			want: Report{Words: 4, Unique: 3, Top: nil, Longest: "the", AverageLength: 2.25},
		},
		{
			name: "TopN zero",
			text: "x y z",
			opts: Options{TopN: 0},
			want: Report{Words: 3, Unique: 3, Top: nil, Longest: "x", AverageLength: 1},
		},
		{
			name: "TopN negative",
			text: "x y z",
			opts: Options{TopN: -1},
			want: Report{Words: 3, Unique: 3, Top: nil, Longest: "x", AverageLength: 1},
		},
		{
			name: "longest counts runes not bytes",
			text: "日本語 abcd",
			opts: Options{},
			// "日本語" is 9 bytes but only 3 runes.
			want: Report{Words: 2, Unique: 2, Longest: "abcd", AverageLength: 3.5},
		},
		{
			name: "longest tie goes to alphabetically first",
			text: "pear plum kiwi fig",
			opts: Options{},
			want: Report{Words: 4, Unique: 4, Longest: "kiwi", AverageLength: 3.75},
		},
		{
			name: "apostrophes count towards length",
			text: "can't stop",
			opts: Options{TopN: 1},
			want: Report{Words: 2, Unique: 2, Top: []WordCount{{"can't", 1}}, Longest: "can't", AverageLength: 4.5},
		},
		{
			name: "ties in top broken alphabetically",
			text: "b a c b a c d",
			opts: Options{TopN: 2},
			want: Report{Words: 7, Unique: 4, Top: []WordCount{{"a", 2}, {"b", 2}}, Longest: "a", AverageLength: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Analyze(tt.text, tt.opts)
			assertReport(t, got, tt.want)
		})
	}
}

func TestAnalyzeDoesNotModifyStopwords(t *testing.T) {
	stop := []string{"The", "AND", "of"}
	before := slices.Clone(stop)
	defaults := slices.Clone(DefaultStopwords)

	Analyze("The cat AND the hat of doom", Options{TopN: 3, Stopwords: stop})
	Analyze("The cat AND the hat of doom", Options{TopN: 3, Stopwords: DefaultStopwords})

	if !slices.Equal(stop, before) {
		t.Errorf("Analyze modified opts.Stopwords: was %q, now %q", before, stop)
	}
	if !slices.Equal(DefaultStopwords, defaults) {
		t.Errorf("Analyze modified DefaultStopwords")
	}
}

// TestAnalyzeDeterministic uses many equal-count words so any dependence on
// map iteration order shows up as differing results.
func TestAnalyzeDeterministic(t *testing.T) {
	var sb strings.Builder
	for i := range 300 {
		fmt.Fprintf(&sb, "word%02d ", i%60) // 60 words x 5 each
	}
	text := sb.String()
	first := Analyze(text, Options{TopN: 5}).String()
	for range 20 {
		if got := Analyze(text, Options{TopN: 5}).String(); got != first {
			t.Fatalf("Analyze is not deterministic; got\n%s\nearlier got\n%s", got, first)
		}
	}
	want := []WordCount{{"word00", 5}, {"word01", 5}, {"word02", 5}, {"word03", 5}, {"word04", 5}}
	if got := Analyze(text, Options{TopN: 5}).Top; !slices.Equal(got, want) {
		t.Errorf("Top = %v, want %v", got, want)
	}
}

func TestReportString(t *testing.T) {
	tests := []struct {
		name string
		r    Report
		want string
	}{
		{
			name: "with top words",
			r: Report{
				Words: 9, Unique: 8, Longest: "jumps", AverageLength: 35.0 / 9,
				Top: []WordCount{{"fox", 2}, {"brown", 1}},
			},
			want: "words: 9\n" +
				"unique: 8\n" +
				"longest: jumps\n" +
				"average length: 3.89\n" +
				"top words:\n" +
				"  1. fox (2)\n" +
				"  2. brown (1)\n",
		},
		{
			name: "zero report",
			r:    Report{},
			want: "words: 0\n" +
				"unique: 0\n" +
				"longest: \n" +
				"average length: 0.00\n" +
				"top words: none\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.String(); got != tt.want {
				t.Errorf("Report.String() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func assertReport(t *testing.T, got, want Report) {
	t.Helper()
	if got.Words != want.Words {
		t.Errorf("Words = %d, want %d", got.Words, want.Words)
	}
	if got.Unique != want.Unique {
		t.Errorf("Unique = %d, want %d", got.Unique, want.Unique)
	}
	if got.Longest != want.Longest {
		t.Errorf("Longest = %q, want %q", got.Longest, want.Longest)
	}
	if math.Abs(got.AverageLength-want.AverageLength) > 1e-9 {
		t.Errorf("AverageLength = %v, want %v", got.AverageLength, want.AverageLength)
	}
	if !(len(got.Top) == 0 && len(want.Top) == 0) && !slices.Equal(got.Top, want.Top) {
		t.Errorf("Top = %v, want %v", got.Top, want.Top)
	}
}
