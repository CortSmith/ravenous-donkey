package semver

import (
	"errors"
	"testing"
)

// FuzzParseFastMatchesParse is a differential test: Parse is the oracle.
//
//	go test ./solutions/04-testing/project/semver/ -run='^$' -fuzz=FuzzParseFastMatchesParse -fuzztime=20s
func FuzzParseFastMatchesParse(f *testing.F) {
	for _, s := range []string{"1.2.3", "v0.1.0", "1.0.0-alpha.1", "2.0.0-rc.1+build.5", "01.2.3", "1.2.3-", "1.2.3-a..b", "1.2", "1.2.3.4", "1.0.0-01"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		want, wantErr := Parse(s)
		got, err := ParseFast(s)
		if got != want || (err == nil) != (wantErr == nil) {
			t.Fatalf("ParseFast(%q) = %+v, %v; Parse = %+v, %v", s, got, err, want, wantErr)
		}
		if err != nil && !errors.Is(err, ErrInvalid) {
			t.Fatalf("ParseFast(%q) error %v does not wrap ErrInvalid", s, err)
		}
	})
}

func TestParseFastDoesNotAllocate(t *testing.T) {
	if n := testing.AllocsPerRun(100, func() { _, _ = ParseFast("v1.22.333-rc.1.alpha+build.2025") }); n != 0 {
		t.Errorf("ParseFast allocated %.1f times per call, want 0", n)
	}
}

func BenchmarkParseReadableVsFast(b *testing.B) {
	const in = "v1.22.333-rc.1.alpha+build.2025"
	b.Run("Readable", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			Parse(in)
		}
	})
	b.Run("Fast", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			ParseFast(in)
		}
	})
}
