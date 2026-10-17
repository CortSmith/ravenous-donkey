package concat

import (
	"fmt"
	"strings"
	"testing"
)

// joiners lists every implementation under test, so each test and
// benchmark covers all of them and a new one only needs adding here.
var joiners = []struct {
	name string
	join func([]string, string) string
}{
	{"Plus", JoinPlus},
	{"Buffer", JoinBuffer},
	{"Builder", JoinBuilder},
}

// TestJoinersMatchStringsJoin uses strings.Join as an oracle: whatever it
// returns is by definition the right answer.
func TestJoinersMatchStringsJoin(t *testing.T) {
	cases := []struct {
		name  string
		elems []string
		sep   string
	}{
		{"nil", nil, ","},
		{"empty slice", []string{}, ","},
		{"one", []string{"a"}, ","},
		{"several", []string{"a", "b", "c"}, ", "},
		{"empty separator", []string{"a", "b"}, ""},
		{"empty first", []string{"", "a"}, ","},
		{"empty middle", []string{"a", "", "b"}, ","},
		{"empty last", []string{"a", ""}, ","},
		{"all empty", []string{"", "", ""}, "-"},
		{"unicode", []string{"héllo", "wörld"}, " ✓ "},
		{"many", makeElems(100, 7), "|"},
	}
	for _, j := range joiners {
		for _, c := range cases {
			t.Run(j.name+"/"+c.name, func(t *testing.T) {
				got, want := j.join(c.elems, c.sep), strings.Join(c.elems, c.sep)
				if got != want {
					t.Errorf("Join%s(%q, %q) = %q, want %q", j.name, c.elems, c.sep, got, want)
				}
			})
		}
	}
}

// TestJoinBuilderAllocatesOnce pins the performance promise in
// JoinBuilder's doc comment, so a regression fails `go test`, not just a
// benchmark nobody reads.
func TestJoinBuilderAllocatesOnce(t *testing.T) {
	elems := makeElems(100, 10)
	allocs := testing.AllocsPerRun(100, func() {
		_ = JoinBuilder(elems, ",")
	})
	if allocs > 1 {
		t.Errorf("JoinBuilder allocated %.1f times per call, want at most 1", allocs)
	}
}

// makeElems returns n strings of the given length, built outside any
// timed region.
func makeElems(n, length int) []string {
	elems := make([]string, n)
	for i := range elems {
		elems[i] = strings.Repeat(string(rune('a'+i%26)), length)
	}
	return elems
}

// BenchmarkJoin compares every implementation at several input sizes.
//
//	go test ./modules/04-testing/exercises/concat/ -run='^$' -bench=. -benchmem
func BenchmarkJoin(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		elems := makeElems(n, 8) // setup: not timed
		for _, j := range joiners {
			b.Run(fmt.Sprintf("%s/n=%d", j.name, n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					j.join(elems, ",")
				}
			})
		}
	}
}

func ExampleJoinBuilder() {
	fmt.Println(JoinBuilder([]string{"a", "", "c"}, "-"))
	// Output: a--c
}
