package textstats

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// AnalyzeFast returns exactly the same Report as Analyze, but tokenizes and
// counts in a single pass with far fewer allocations:
//
//   - No []rune copy of the whole text (4 bytes per rune) and no []string of
//     every token: each word is counted the moment it ends.
//   - When a word in the input is already in canonical form (lowercase, no
//     curly apostrophe) the map key is a substring of text: zero copies. Only
//     words that need rewriting are built in a reused scratch buffer and
//     looked up in an intern map; the compiler turns a read m[string(b)]
//     into a lookup without allocating, so a string is only allocated the
//     first time each distinct rewritten word is seen.
//   - Rune counts and the longest word are computed once per distinct word,
//     not once per occurrence.
//
// Map keys that are substrings of text keep text alive while the map exists.
// The returned Report clones the few strings it holds, so the caller's text
// can be garbage collected afterwards.
func AnalyzeFast(text string, opts Options) Report {
	counts := make(map[string]int)
	interned := make(map[string]string) // rewritten word -> its one allocated copy
	var scratch []byte
	words := 0
	start := -1       // byte offset where the current word began, or -1
	canonical := true // current word can be used as-is (a substring of text)

	flush := func(end int) {
		words++
		if canonical {
			counts[text[start:end]]++
			return
		}
		key, ok := interned[string(scratch)] // a read: no allocation
		if !ok {
			key = string(scratch) // first sighting: allocate once
			interned[key] = key
		}
		counts[key]++
	}

	for i := 0; i < len(text); {
		r, size := rune(text[i]), 1
		if r >= utf8.RuneSelf {
			r, size = utf8.DecodeRuneInString(text[i:])
		}
		switch {
		case isWordRune(r):
			if start < 0 {
				start, canonical, scratch = i, true, scratch[:0]
			}
			lr := unicode.ToLower(r)
			if canonical && lr != r {
				// First rune that needs rewriting: copy the prefix so far.
				canonical = false
				scratch = append(scratch[:0], text[start:i]...)
			}
			if !canonical {
				scratch = utf8.AppendRune(scratch, lr)
			}
		case isApostrophe(r) && start >= 0 && nextIsWordRune(text, i+size):
			if canonical && r != '\'' {
				canonical = false
				scratch = append(scratch[:0], text[start:i]...)
			}
			if !canonical {
				scratch = append(scratch, '\'')
			}
		case start >= 0:
			flush(i)
			start = -1
		}
		i += size
	}
	if start >= 0 {
		flush(len(text))
	}

	r := Report{Words: words, Unique: len(counts)}
	if words == 0 {
		return r
	}
	totalRunes, longestLen := 0, 0
	for w, c := range counts {
		n := utf8.RuneCountInString(w)
		totalRunes += n * c
		if n > longestLen || (n == longestLen && w < r.Longest) {
			r.Longest, longestLen = w, n
		}
	}
	r.Longest = strings.Clone(r.Longest) // don't pin the whole input text
	r.AverageLength = float64(totalRunes) / float64(words)
	r.Top = topWords(counts, stopSet(opts.Stopwords), opts.TopN)
	for i := range r.Top {
		r.Top[i].Word = strings.Clone(r.Top[i].Word)
	}
	return r
}

func nextIsWordRune(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return isWordRune(r)
}
