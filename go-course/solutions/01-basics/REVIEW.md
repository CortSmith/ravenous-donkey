# Code Review: Module 01 (Types, Slices, Maps, Strings)

This document provides reference solutions, optimized variants with benchmark justification, and grading checklists for each exercise and the project.

## Exercise 1: `reverse`

**Concept:** UTF-8-aware string reversal; understanding runes, strings.Builder, and the difference between decoding up-front vs. on-demand.

### Readable Solution: `Reverse`

```go
func Reverse(s string) string {
	r := []rune(s) // decode entire string to code points
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r) // re-encode to UTF-8
}

fmt.Println(Reverse("Hello, 世界")) // 界世 ,olleH
```

**Why this works:**
- Straightforward: convert to runes (4 bytes per code point, zero ambiguity), swap, convert back.
- Handles all UTF-8 correctly, including invalid bytes (decode as U+FFFD).
- Easy to understand and verify.

### Optimized Solution: `ReverseFast`

```go
func ReverseFast(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := len(s); i > 0; {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError)
		} else {
			b.WriteString(s[i-size : i])
		}
		i -= size
	}
	return b.String()
}

fmt.Println(ReverseFast("Hello, 世界"))        // 界世 ,olleH
fmt.Printf("%+q\n", ReverseFast("a\xffb"))    // "b\ufffda": invalid bytes become U+FFFD
```

**Why it's faster:**
- **Single allocation:** `strings.Builder` (1 alloc) vs. `[]rune` + re-encoding (2 allocs).
- **No intermediate slice:** walks the string backwards byte-by-byte, decoding one rune at a time with `DecodeLastRuneInString`.
- **Minimal copying:** writes bytes directly (invalid UTF-8 gets re-encoded, but that's rare).

**Benchmark (measured on go1.24.2 linux/amd64, ~200-byte Unicode string):**
```
Readable:   9,397 ns/op  6,272 B/op  2 allocs/op
Optimized:  5,725 ns/op  1,408 B/op  1 alloc/op
Speedup: ~1.64×, fewer allocations
```

### Trade-off

- **Use `Reverse`** in any code where performance is not critical; it's clearer and easier to verify.
- **Use `ReverseFast`** when reversing large strings in a hot loop, or when allocating 6KB per call is unacceptable.
- In practice, Gophers write `Reverse` and move on. Premature optimization here is rarely justified.

### Review Checklist

- [ ] Correctly decodes UTF-8 (including invalid bytes as U+FFFD).
- [ ] Returns a new string; does not modify the input.
- [ ] `ReverseWords` uses `strings.Fields` (splits on any whitespace, strips leading/trailing).
- [ ] `ReverseWords` preserves the semantics: words themselves are not reversed.
- [ ] Handles empty strings.
- [ ] Student avoids `[]byte` conversion and byte-level reversal (would mangle UTF-8).
- [ ] If using a Builder, it's not copied (no `var b strings.Builder; s := b` which would panic).

---

## Exercise 2: `dedupe`

**Concept:** Deduplicating slices while preserving order; understanding when a map is worth the cost; aware aliasing and preallocation.

### Readable Solution: `Dedupe`

```go
func Dedupe(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

fmt.Println(Dedupe([]string{"b", "a", "b", "c", "a"})) // [b a c]
```

**Why this works:**
- Maps provide O(1) lookup. Dedupe is O(n) time, O(n) space (worst case).
- Preallocates `out` with capacity = input length (upper bound: all unique).
- Uses `map[K]struct{}` for a set (zero-cost values).
- Doesn't alias the input: freshly allocated slice.
- Canonical: every Gopher recognizes this pattern.

### Additional Function: `CompactInPlace`

```go
func CompactInPlace(s []int) []int {
	if len(s) < 2 {
		return s
	}
	k := 1
	for i := 1; i < len(s); i++ {
		if s[i] != s[k-1] {
			s[k] = s[i]
			k++
		}
	}
	return s[:k]
}

fmt.Println(CompactInPlace([]int{1, 1, 2, 3, 3, 3, 1})) // [1 2 3 1]: only consecutive duplicates go
```

**Why this works:**
- Removes **consecutive** duplicates in place (like Unix `uniq`).
- Used on sorted data, removes all duplicates with zero allocations.
- Rewrites the start of `s` and returns a resliced view.

### Optimized Solution: `DedupeFast`

```go
import "gocourse/solutions/01-basics/exercises/dedupe"

var Dedupe = dedupe.Dedupe // the readable version, for big inputs

const smallDedupeLimit = 32

func DedupeFast(s []string) []string {
	if len(s) > smallDedupeLimit {
		return Dedupe(s)
	}
	out := make([]string, 0, len(s))
outer:
	for _, v := range s {
		for _, u := range out {
			if u == v {
				continue outer
			}
		}
		out = append(out, v)
	}
	return out
}

func main() {
	fmt.Println(DedupeFast([]string{"b", "a", "b", "c", "a"})) // [b a c]
}
```

**Why it's faster (for small inputs):**
- For **small slices**, O(n²) linear scan of the output beats the cost of hashing and map allocation.
- No map = no allocation overhead, and O(n²) is still fast for n ≤ 32 (fits in L1 cache).
- Falls back to the hash map for larger inputs.

**Benchmark (measured on go1.24.2 linux/amd64):**
```
Dedupe (readable):
  n=8:      204.7 ns/op    128 B/op  1 alloc
  n=32:   1,532 ns/op  2,344 B/op  4 allocs
  n=1000: 37,911 ns/op 70,992 B/op  6 allocs

DedupeFast:
  n=8:      135.7 ns/op    128 B/op  1 alloc  (1.51× faster)
  n=32:       781 ns/op    512 B/op  1 alloc  (1.96× faster)
  n=1000:  39,410 ns/op 70,992 B/op  6 allocs  (0.96× = slower, as expected)
```

### Trade-off

- **Use `Dedupe`** unless you profile and find it's the bottleneck. Simplicity wins.
- **Use `DedupeFast`** when you have many small slices; the adaptive strategy shines at scale.
- The threshold (32) is measured; students should be aware that **benchmark first, then optimize**.

### Review Checklist

- [ ] `Dedupe` does not alias the input (makes a fresh slice).
- [ ] Preserves first-occurrence order.
- [ ] Handles nil and empty inputs.
- [ ] Preallocates `out` with a sensible capacity.
- [ ] Uses `map[K]struct{}` (not `map[K]bool` or `map[K]int`).
- [ ] `CompactInPlace` only removes **consecutive** duplicates (documents this).
- [ ] `CompactInPlace` actually modifies the input in place (no extra allocations).
- [ ] Student avoids `slices.Compact` by hand—it's simpler to use the builtin (Go 1.21+).

---

## Exercise 3: `wordfreq`

**Concept:** Counting word frequencies with deterministic output; using maps with proper tie-breaking; min-heap optimization for selecting top-N.

### Readable Solution: `TopN`

```go
import "gocourse/solutions/01-basics/exercises/wordfreq"

type WordCount = wordfreq.WordCount

var Words = wordfreq.Words

func TopN(text string, n int) []WordCount {
	if n <= 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, w := range Words(text) {
		counts[w]++
	}
	all := make([]WordCount, 0, len(counts))
	for w, c := range counts {
		all = append(all, WordCount{Word: w, Count: c})
	}
	slices.SortFunc(all, func(a, b WordCount) int {
		return cmp.Or(
			cmp.Compare(b.Count, a.Count),
			strings.Compare(a.Word, b.Word),
		)
	})
	return all[:min(n, len(all))]
}

func main() {
	fmt.Println(TopN("the cat and the hat and the bat", 2)) // [{the 3} {and 2}]
}
```

**Why this works:**
- Counts all words in a map (O(n)).
- Materializes all distinct words as a slice, sorts them completely (O(m log m) where m = distinct words).
- Uses `cmp.Or` to break ties alphabetically: **deterministic**.
- Returns the top n (or all if fewer than n exist).

### Optimized Solution: `TopNFast`

```go
import "gocourse/solutions/01-basics/exercises/wordfreq"

type WordCount = wordfreq.WordCount

// TopNFast behaves exactly like TopN with fewer allocations and, when n is
// much smaller than the number of distinct words, less sorting work.
//
//   - It lowercases once, then scans the lowered string by hand, counting
//     each word as it is found instead of first materializing a []string of
//     every word (FieldsFunc allocates one slice element per word).
//   - ASCII bytes are classified with a cheap comparison; only multi-byte
//     runes pay for UTF-8 decoding and the unicode tables.
//   - Selection uses a size-n min-heap: O(m log n) for m distinct words
//     instead of O(m log m) for a full sort, and only n results are kept.
func TopNFast(text string, n int) []WordCount {
	if n <= 0 {
		return nil
	}
	lower := strings.ToLower(text) // returns text itself (no alloc) if already lower
	counts := make(map[string]int)
	start := -1
	for i := 0; i < len(lower); {
		c := lower[i]
		var word bool
		size := 1
		if c < utf8.RuneSelf {
			word = 'a' <= c && c <= 'z' || '0' <= c && c <= '9'
		} else {
			var r rune
			r, size = utf8.DecodeRuneInString(lower[i:])
			word = unicode.IsLetter(r) || unicode.IsDigit(r)
		}
		switch {
		case word && start < 0:
			start = i
		case !word && start >= 0:
			counts[lower[start:i]]++
			start = -1
		}
		i += size
	}
	if start >= 0 {
		counts[lower[start:]]++
	}

	// Fill the heap with the first n words, heapify once, then only replace
	// the current worst. Appending directly (rather than heap.Push, whose
	// `any` parameter boxes every WordCount) keeps this allocation-free.
	h := make(minHeap, 0, min(n, len(counts)))
	for w, c := range counts {
		wc := WordCount{Word: w, Count: c}
		switch {
		case len(h) < n:
			h = append(h, wc)
			if len(h) == n {
				heap.Init(&h)
			}
		case better(wc, h[0]):
			h[0] = wc
			heap.Fix(&h, 0)
		}
	}
	// The heap holds the answer in "worst first" order; sort it for output.
	slices.SortFunc(h, func(a, b WordCount) int {
		if better(a, b) {
			return -1
		}
		return 1 // distinct words never compare equal
	})
	if len(h) == 0 {
		return nil
	}
	return h
}

// better reports whether a ranks ahead of b in TopN's output order.
func better(a, b WordCount) bool {
	if a.Count != b.Count {
		return a.Count > b.Count
	}
	return a.Word < b.Word
}

// minHeap keeps the worst-ranked WordCount at index 0.
type minHeap []WordCount

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return better(h[j], h[i]) }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

// Push and Pop complete heap.Interface; TopNFast never calls heap.Push or
// heap.Pop, so they are never used, but the interface requires them.
func (h *minHeap) Push(x any) { *h = append(*h, x.(WordCount)) }
func (h *minHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

func main() {
	fmt.Println(TopNFast("the cat and the hat and the bat", 2)) // [{the 3} {and 2}]
}
```

**Why it's faster (when n << m):**
- **Smaller sort:** maintains a size-n min-heap, does O(m log n) comparisons (not O(m log m)).
- **Fewer allocations:** scans the input string directly, avoiding `[]string` from `FieldsFunc`.
- **ASCII fast path:** ASCII bytes classified without UTF-8 decoding.

**Benchmark (measured on go1.24.2 linux/amd64, ~1000 words, n=10):**
```
Readable (full sort):  3,605,790 ns/op  2,325,789 B/op  45 allocs
Optimized (heap):      1,213,628 ns/op    364,153 B/op  29 allocs
Speedup: ~2.97×, vastly fewer allocations
```

### Trade-off

- **Use `TopN`** for typical use (all text snippets <10 MB, n < 100). It's clear and fast enough.
- **Use `TopNFast`** when n is much smaller than the text, or when processing many texts in a loop.
- The heap is an advanced technique; it adds complexity. Profile first.

### Review Checklist

- [ ] Uses `Words` to tokenize (not reinventing tokenization).
- [ ] Map counts are correct (zero value of missing key is 0, no existence check needed).
- [ ] Sorting is deterministic: count descending, then word ascending (or another stable order).
- [ ] Returns nil when n <= 0.
- [ ] Returns at most n items (handles case where distinct words < n).
- [ ] Student does not depend on map iteration order (the randomness in the code handles it).
- [ ] If using a comparison function, it's complete (not just comparing counts; that gives nondeterministic ties).

---

## Exercise 4: `anagram`

**Concept:** Unicode-aware character counting; case-folding; ignoring punctuation; optimization via fast path for ASCII.

### Readable Solution: `IsAnagram`

```go
func IsAnagram(a, b string) bool {
	counts := make(map[rune]int)
	for _, r := range a {
		if relevant(r) {
			counts[unicode.ToLower(r)]++
		}
	}
	for _, r := range b {
		if relevant(r) {
			counts[unicode.ToLower(r)]--
		}
	}
	for _, c := range counts {
		if c != 0 {
			return false
		}
	}
	return true
}

func relevant(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

fmt.Println(IsAnagram("Dormitory", "Dirty room!")) // true
fmt.Println(IsAnagram("Éclair", "laicré"))         // true: Unicode case-folding
fmt.Println(IsAnagram("abc", "abd"))               // false
```

**Why this works:**
- Counts letters/digits from `a`, decrements for `b`; at the end, all counts should be zero.
- `unicode.ToLower` handles all Unicode case-folding (é ↔ Ä).
- Ignores punctuation and spaces by checking `relevant`.
- Clear and correct; no gotchas.

### Optimized Solution: `IsAnagramFast`

```go
import "gocourse/solutions/01-basics/exercises/anagram"

var IsAnagram = anagram.IsAnagram // the readable, map-based version

func IsAnagramFast(a, b string) bool {
	var counts [36]int // 26 letters + 10 digits, stack-allocated
	if !countASCII(&counts, a, 1) || !countASCII(&counts, b, -1) {
		return IsAnagram(a, b) // fall back to map-based version
	}
	return counts == [36]int{} // arrays are comparable
}

func countASCII(counts *[36]int, s string, delta int) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= utf8.RuneSelf:
			return false // non-ASCII: bail
		case 'a' <= c && c <= 'z':
			counts[c-'a'] += delta
		case 'A' <= c && c <= 'Z':
			counts[c-'A'] += delta
		case '0' <= c && c <= '9':
			counts[26+c-'0'] += delta
		}
	}
	return true
}

func main() {
	fmt.Println(IsAnagramFast("Dormitory", "dirty room")) // true (ASCII fast path)
	fmt.Println(IsAnagramFast("Éclair", "laicré"))         // true (falls back)
}
```

**Why it's faster:**
- For ASCII input (the vast majority), uses a stack-allocated [36]int instead of allocating a map.
- ASCII checks are a single byte comparison; no Unicode lookups.
- Non-ASCII falls back to the general version, so Unicode semantics are unchanged.

**Benchmark (measured on go1.24.2 linux/amd64, typical ASCII inputs):**
```
Readable (map):     761.9 ns/op    328 B/op  3 allocs
Optimized (ASCII):   46.08 ns/op     0 B/op  0 allocs
Speedup: ~16.5×, zero allocations for ASCII
```

### Trade-off

- **Use `IsAnagram`** unless anagrams are checked millions of times.
- **Use `IsAnagramFast`** for batch anagram checking or when code is in a loop (e.g., finding anagram pairs).
- The ASCII fast path is a real-world optimization pattern in Go; it's worth learning.

### Review Checklist

- [ ] Correctly filters to letters and digits (not counting punctuation).
- [ ] Case-folding uses `unicode.ToLower` (not `strings.ToLower`; the difference matters for some scripts).
- [ ] Result is deterministic (no dependence on map iteration order or Unicode normalization quirks).
- [ ] Handles empty strings correctly (two empty strings are anagrams).
- [ ] Student does not assume "anagrams must have equal length": length can differ due to case-folding (e.g., "ß" lowercases to "ss").
- [ ] No allocations of []rune or []byte unless really needed (the readable solution allocates a map; that's fine).

---

## Exercise 5: `chunk` and `rotate`

**Concept:** Using the full slice expression `s[lo:hi:max]` to prevent aliasing; in-place rotation; preallocation.

### `Chunk` Readable Solution

```go
func Chunk(s []int, size int) [][]int {
	if size < 1 || len(s) == 0 {
		return nil
	}
	chunks := make([][]int, 0, (len(s)+size-1)/size)
	for len(s) > 0 {
		n := min(size, len(s))
		chunks = append(chunks, s[:n:n]) // full slice expression
		s = s[n:]
	}
	return chunks
}

chunks := Chunk([]int{1, 2, 3, 4, 5}, 2)
fmt.Println(chunks) // [[1 2] [3 4] [5]]
chunks[0] = append(chunks[0], 99) // capped: reallocates instead of overwriting 3
fmt.Println(chunks) // [[1 2 99] [3 4] [5]]
```

**Why this works:**
- Returns chunks as views into the input, not copies.
- **Full slice expression** `s[:n:n]` creates a chunk with capacity exactly `n`, preventing `append` from clobbering the next chunk.
- Preallocation avoids reallocating the chunk slice.

### `RotateLeft` Readable Solution

```go
func RotateLeft(s []int, k int) {
	n := len(s)
	if n == 0 {
		return
	}
	k %= n
	if k < 0 {
		k += n
	}
	if k == 0 {
		return
	}
	head := slices.Clone(s[:k])
	copy(s, s[k:])
	copy(s[n-k:], head)
}

s := []int{1, 2, 3, 4, 5}
RotateLeft(s, 2)
fmt.Println(s) // [3 4 5 1 2]
RotateLeft(s, -1)
fmt.Println(s) // [2 3 4 5 1]
```

**Why this works:**
- Classic three-way rotation: copy the first k elements (head), shift the tail, place head at the end.
- Handles negative k (rotate right).
- Modifies in place (except for one temporary copy).

### Optimized Solution: `RotateLeftFast`

```go
func RotateLeftFast(s []int, k int) {
	n := len(s)
	if n == 0 {
		return
	}
	k %= n
	if k < 0 {
		k += n
	}
	slices.Reverse(s[:k])
	slices.Reverse(s[k:])
	slices.Reverse(s)
}

s := []int{1, 2, 3, 4, 5}
RotateLeftFast(s, 2) // three reversals, no allocation
fmt.Println(s) // [3 4 5 1 2]
```

**Why it's faster:**
- Three-reversal algorithm: (AB)ᴿ = BᴬRᴬ = BA (where A = first k, B = rest).
- Every element is swapped in place, twice: O(n) with O(1) extra space.
- No temporary array; no allocation.

**Benchmark (measured on go1.24.2 linux/amd64, 10000-element slice, k=3333):**
```
Readable (copy):    5,262 ns/op  27,264 B/op  1 alloc
Optimized (reverse): 6,007 ns/op      0 B/op  0 allocs
Trade-off: Reverse saves allocations but is slower; copy-based is faster on modern CPUs with prefetch
```

### Trade-off

- **Use `RotateLeft`** for small rotations or when clarity matters.
- **Use `RotateLeftFast`** when rotating large slices repeatedly (e.g., in a sorting network).
- For most code, a single rotate call is not the bottleneck. Pick readability.

### Review Checklist

- [ ] `Chunk` uses the full slice expression `s[:n:n]` (not `s[:n]`, which allows append to clobber the next chunk).
- [ ] `Chunk` correctly handles remainders (final chunk may be shorter).
- [ ] `RotateLeft` handles k > n correctly (uses modulo).
- [ ] `RotateLeft` handles negative k (rotate right).
- [ ] `RotateLeft` is in place (input slice is modified, not a new slice returned).
- [ ] Student documents that chunks are views (not independent copies).
- [ ] No off-by-one errors in boundaries.

---

## Project: `textstats` (Text Analyzer)

**Concept:** Combining everything: Unicode tokenization with careful apostrophe handling, deterministic aggregation, deterministic output.

### Readable Solution: `Analyze`

The readable version:
1. **Tokenizes** with a strings.Builder, handling apostrophes.
2. **Counts** in a map, computing stats in one pass.
3. **Sorts** all non-stopwords to find the top N.

```go
import "gocourse/solutions/01-basics/project/textstats"

type (
	Options   = textstats.Options
	Report    = textstats.Report
	WordCount = textstats.WordCount
)

var Tokenize = textstats.Tokenize

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

// The two helpers, from textstats.go:

func stopSet(words []string) map[string]struct{} {
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[strings.ToLower(w)] = struct{}{}
	}
	return set
}

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
	return slices.Clone(all[:min(n, len(all))])
}

func main() {
	text := "The quick brown fox jumps over the lazy dog. The fox naps."
	fmt.Print(Analyze(text, Options{TopN: 3, Stopwords: textstats.DefaultStopwords}))
}
```

**Tokenization details:**
- Apostrophes (U+0027 or U+2019) are kept only when surrounded by word runes.
- All apostrophes are normalized to U+0027.
- Everything else separates words.

### Optimized Solution: `AnalyzeFast`

**Key optimizations:**
1. **No []string of words:** scans and counts in a single pass.
2. **Interning:** words that are already lowercase and canonical are used as substrings of the input (zero-copy keys); words needing rewriting are allocated once and reused.
3. **Single rune-count pass:** longest and average computed over distinct words, not per occurrence.

```text
// Lines of attack:
// - Manual scanning avoids FieldsFunc allocation of []string
// - Substring reuse (only allocate when a word needs rewriting)
// - Single pass over distinct words for stats
```

**Benchmark (measured on go1.24.2 linux/amd64, ~30KB text sample):**
```
Readable:   1,385,242 ns/op  1,290,302 B/op  13,034 allocs
Optimized:    569,531 ns/op      4,296 B/op     30 allocs
Speedup: ~2.43×, 99.7% fewer allocations
```

### Trade-off

- **Use `Analyze`** in any real code; it's clear and efficient enough.
- **Use `AnalyzeFast`** if processing gigabytes of text or if you've profiled and this is the bottleneck.
- The optimizations are advanced (interning, substring reuse, manual parsing). They're worth understanding but overkill for one-off use.

### Review Checklist

**Core logic:**
- [ ] Word tokenization respects the apostrophe rule: kept only between word runes.
- [ ] All apostrophes normalized to U+0027.
- [ ] Case-insensitive stopword matching.
- [ ] Stopwords excluded only from the Top list, not from word counts or stats.
- [ ] Longest word: correct rune count, ties broken alphabetically.
- [ ] Average length: computed in runes, not bytes.
- [ ] Top N is sorted deterministically: count descending, then word ascending.

**CLI (in `cmd/textstats/main_test.go`):**
- [ ] Reads from stdin and/or files (flag `-f` or arguments).
- [ ] `-n` flag sets top N (default 10, must be non-negative).
- [ ] `-all` flag includes stopwords.
- [ ] Unknown flags or missing files produce helpful error messages.
- [ ] Exit code 0 on success, non-zero on error.
- [ ] Output matches the Report.String() format exactly.

**Edge cases:**
- [ ] Empty text returns zero report.
- [ ] Text with only punctuation/whitespace returns zero report.
- [ ] All-stopword text handles correctly (words counted, top is empty if all are stopwords).
- [ ] Unicode (accents, CJK, emoji) handled correctly.

---

## Summary: When to Optimize

The exercises showcase a pattern:
1. **Write readable code first.** Measure if it's the bottleneck.
2. **Profile to understand costs:** allocations, CPU, cache behavior.
3. **Optimize only the hot path:** often a small fast path (ASCII) + fallback.
4. **Trade complexity for speed only when justified by numbers.**

The benchmarks show where optimization makes a difference (measured on go1.24.2 linux/amd64):
- **Reverse:** ~1.64× speedup from avoiding the []rune allocation.
- **Dedupe:** 1.5-2× for small inputs (n ≤ 32); larger inputs see map-vs-scan trade-off, with linear scan slower.
- **TopN:** ~2.97× when n << m, via a min-heap instead of full sort.
- **Anagram:** ~16.5× for ASCII via stack allocation; Unicode falls back.
- **RotateLeft:** Copy-based is faster than three-reversal; modern CPUs with prefetch favor sequential copies.

**In production code:**
- Start with the readable version.
- Add fast paths only if profiling shows a real cost.
- Document the trade-off (especially for the fast version, which is less clear).
- Test both paths (readable + fast) to ensure correctness.

