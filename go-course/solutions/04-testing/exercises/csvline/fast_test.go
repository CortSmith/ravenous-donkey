package csvline

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// FuzzFastMatchesReadable is a differential test: the readable functions
// are the oracle for the fast ones.
//
//	go test ./solutions/04-testing/exercises/csvline/ -run='^$' -fuzz=FuzzFastMatchesReadable -fuzztime=20s
func FuzzFastMatchesReadable(f *testing.F) {
	for _, seed := range []string{"", "a,b,c", `"a,b",c`, `x,"say ""hi""",z`, ",,", `"abc`, `"a"b`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, line string) {
		want, wantErr := Split(line)
		got, err := SplitFast(line)
		if !slices.Equal(got, want) || errors.Is(err, ErrSyntax) != errors.Is(wantErr, ErrSyntax) ||
			errors.Is(err, ErrUnterminated) != errors.Is(wantErr, ErrUnterminated) {
			t.Fatalf("SplitFast(%q) = %q, %v; Split = %q, %v", line, got, err, want, wantErr)
		}
		// Reuse the fuzzed line as field contents for the Join side.
		fields := strings.Split(line, "|")
		if got, want := JoinFast(fields), Join(fields); got != want {
			t.Fatalf("JoinFast(%q) = %q, Join = %q", fields, got, want)
		}
	})
}

func TestJoinFastAllocatesOnce(t *testing.T) {
	fields := []string{"plain", `say "hi"`, "a,b", "two\nlines"}
	if n := testing.AllocsPerRun(100, func() { _ = JoinFast(fields) }); n > 1 {
		t.Errorf("JoinFast allocated %.1f times per call, want at most 1", n)
	}
}

var benchFields = []string{"id-123", "Ada Lovelace", `said "hello, world"`, "", "London", "1815-12-10"}

func BenchmarkJoin(b *testing.B) {
	b.Run("Readable", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			Join(benchFields)
		}
	})
	b.Run("Fast", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			JoinFast(benchFields)
		}
	})
}

func BenchmarkSplit(b *testing.B) {
	line := Join(benchFields)
	b.Run("Readable", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			Split(line)
		}
	})
	b.Run("Fast", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			SplitFast(line)
		}
	})
}
