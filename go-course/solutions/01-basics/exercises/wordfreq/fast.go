package wordfreq

import (
	"container/heap"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

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
