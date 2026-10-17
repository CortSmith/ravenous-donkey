package textstats

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

func TestAnalyzeFastMatchesAnalyze(t *testing.T) {
	texts := []string{
		"", "...", sample, "Don’t DON'T don't", "rock'n'roll 'tis dogs' a''b",
		"Ça va, Zoë? Ünïcödé!", "日本語 テキスト 日本語", "x\xffy", "İstanbul ISTANBUL",
		"the cat and the hat", "ΣΊΣΥΦΟΣ σίσυφος",
	}
	rng := rand.New(rand.NewPCG(7, 8))
	vocab := []string{"Go", "go", "don't", "Don’t", "ÉLAN", "élan", "日本", "42", "the", "A", "'", "’", "-", " ", "\n", "!"}
	for range 500 {
		var sb strings.Builder
		for range rng.IntN(40) {
			sb.WriteString(vocab[rng.IntN(len(vocab))])
			if rng.IntN(3) == 0 {
				sb.WriteByte(' ')
			}
		}
		texts = append(texts, sb.String())
	}
	for _, text := range texts {
		for _, opts := range []Options{{TopN: 3}, {TopN: 10, Stopwords: DefaultStopwords}, {}} {
			got, want := AnalyzeFast(text, opts), Analyze(text, opts)
			if len(got.Top) == 0 && len(want.Top) == 0 {
				got.Top, want.Top = nil, nil
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("AnalyzeFast(%q, %+v) =\n%+v\nwant\n%+v", text, opts, got, want)
			}
		}
	}
}

var benchText = strings.Repeat(sample+" Gophers don’t panic; they return errors. ", 500)

func BenchmarkAnalyze(b *testing.B) {
	b.ReportAllocs()
	opts := Options{TopN: 10, Stopwords: DefaultStopwords}
	for b.Loop() {
		Analyze(benchText, opts)
	}
}

func BenchmarkAnalyzeFast(b *testing.B) {
	b.ReportAllocs()
	opts := Options{TopN: 10, Stopwords: DefaultStopwords}
	for b.Loop() {
		AnalyzeFast(benchText, opts)
	}
}
