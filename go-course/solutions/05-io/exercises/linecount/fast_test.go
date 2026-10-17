package linecount

import (
	"bufio"
	"strings"
	"testing"
	"testing/iotest"
)

func TestCountFastMatchesCount(t *testing.T) {
	pieces := []string{"", "a", "bc", "\n", "\r", "\r\n", "def", "\n\n", "日", strings.Repeat("w", 70000)}
	for _, p1 := range pieces {
		for _, p2 := range pieces {
			for _, p3 := range pieces {
				in := p1 + p2 + p3
				want, _ := Count(strings.NewReader(in))
				got, err := CountFast(iotest.HalfReader(strings.NewReader(in)))
				if err != nil {
					t.Fatalf("CountFast error = %v", err)
				}
				if got != want {
					t.Errorf("CountFast(%.20q...) = %+v, want %+v", in, got, want)
				}
			}
		}
	}
}

var benchInput = strings.Repeat("127.0.0.1 - - [10/Oct/2026:13:55:36 +0000] \"GET /index.html HTTP/1.1\" 200 2326\r\n", 50000)

func BenchmarkCount(b *testing.B) {
	b.SetBytes(int64(len(benchInput)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Count(strings.NewReader(benchInput)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCountFast(b *testing.B) {
	b.SetBytes(int64(len(benchInput)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := CountFast(strings.NewReader(benchInput)); err != nil {
			b.Fatal(err)
		}
	}
}

// countScanner is the "obvious" implementation, for comparison only. It must
// raise the token limit and so holds the longest line in memory.
func countScanner(in string) Stats {
	sc := bufio.NewScanner(strings.NewReader(in))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<30)
	var s Stats
	for sc.Scan() {
		s.Lines++
		s.Longest = max(s.Longest, len(sc.Bytes()))
	}
	return s
}

func BenchmarkCountScanner(b *testing.B) {
	b.SetBytes(int64(len(benchInput)))
	b.ReportAllocs()
	for b.Loop() {
		countScanner(benchInput)
	}
}
