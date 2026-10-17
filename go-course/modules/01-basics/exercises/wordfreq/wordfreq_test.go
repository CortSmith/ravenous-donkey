package wordfreq

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestWords(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ...  !!", nil},
		{"Hello", []string{"hello"}},
		{"Hello, World!", []string{"hello", "world"}},
		{"one,two;three--four", []string{"one", "two", "three", "four"}},
		{"Go 1.24 rocks", []string{"go", "1", "24", "rocks"}},
		{"ÉCOLE École", []string{"école", "école"}},
		{"tab\tnew\nline", []string{"tab", "new", "line"}},
		{"日本 語", []string{"日本", "語"}},
		{"don't", []string{"don", "t"}}, // apostrophes separate words here
	}
	for _, tt := range tests {
		got := Words(tt.in)
		if len(got) == 0 && len(tt.want) == 0 {
			continue // nil and empty are both fine
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("Words(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTopN(t *testing.T) {
	const fox = "The quick brown fox jumps over the lazy dog. The dog sleeps; the fox runs!"
	tests := []struct {
		name string
		text string
		n    int
		want []WordCount
	}{
		{"n zero", fox, 0, nil},
		{"n negative", fox, -3, nil},
		{"empty text", "", 5, nil},
		{"only punctuation", "?! -- ...", 3, nil},
		{
			"basic top 3", fox, 3,
			[]WordCount{{"the", 4}, {"dog", 2}, {"fox", 2}},
		},
		{
			"ties broken alphabetically", "pear apple fig apple pear fig banana", 4,
			[]WordCount{{"apple", 2}, {"fig", 2}, {"pear", 2}, {"banana", 1}},
		},
		{
			"n larger than distinct words", "b a b", 10,
			[]WordCount{{"b", 2}, {"a", 1}},
		},
		{
			"case insensitive", "Go GO go gO", 1,
			[]WordCount{{"go", 4}},
		},
		{
			"unicode words", "Ça va? ça VA! ÇA", 2,
			[]WordCount{{"ça", 3}, {"va", 2}},
		},
		{
			"digits count as words", "2024 2025 2024", 2,
			[]WordCount{{"2024", 2}, {"2025", 1}},
		},
		{
			"cut inside a tie", "c b a c b a d", 2,
			[]WordCount{{"a", 2}, {"b", 2}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TopN(tt.text, tt.n)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("TopN(%q, %d) =\n  %v\nwant\n  %v", tt.text, tt.n, got, tt.want)
			}
		})
	}
}

func TestTopNNilForNonPositiveN(t *testing.T) {
	for _, n := range []int{0, -1} {
		if got := TopN("a b c", n); got != nil {
			t.Errorf("TopN(%q, %d) = %v, want nil", "a b c", n, got)
		}
	}
}

// TestTopNDeterministic guards against relying on map iteration order, which
// Go deliberately randomizes. Many distinct words with equal counts make an
// unsorted or partially sorted result very likely to differ between runs.
func TestTopNDeterministic(t *testing.T) {
	var sb strings.Builder
	for i := range 200 {
		fmt.Fprintf(&sb, "w%03d ", i%50) // 50 words, each exactly 4 times
	}
	text := sb.String()
	first := TopN(text, 10)
	want := make([]WordCount, 10)
	for i := range want {
		want[i] = WordCount{Word: fmt.Sprintf("w%03d", i), Count: 4}
	}
	if !slices.Equal(first, want) {
		t.Fatalf("TopN with 50 four-way ties = %v, want %v", first, want)
	}
	for range 20 {
		if got := TopN(text, 10); !slices.Equal(got, first) {
			t.Fatalf("TopN is not deterministic: got %v, earlier got %v", got, first)
		}
	}
}
