package configparse

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"
)

// TestParseFastMatchesParse runs both implementations over the same inputs
// and requires identical results.
func TestParseFastMatchesParse(t *testing.T) {
	inputs := []string{
		"", "# c\n\n", "a=1\nb = 2\r\n", "dsn=x=y", "bad", "=v", "my key=1", "a=1\na=2",
		benchInput(50),
	}
	for _, in := range inputs {
		want, wantErr := Parse(strings.NewReader(in))
		got, gotErr := ParseFast(strings.NewReader(in))
		if !maps.Equal(got, want) {
			t.Errorf("ParseFast(%q) map = %v, want %v", in, got, want)
		}
		if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
			t.Errorf("ParseFast(%q) err = %v, want %v", in, gotErr, wantErr)
		}
		var pe *ParseError
		if errors.As(wantErr, &pe) != errors.As(gotErr, &pe) {
			t.Errorf("ParseFast(%q) err type %T, want %T", in, gotErr, wantErr)
		}
	}
}

func benchInput(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "# setting %d\nsection.key_%d = some value %d\n\n", i, i, i)
	}
	return b.String()
}

func BenchmarkParse(b *testing.B) {
	in := benchInput(200)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(strings.NewReader(in)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFast(b *testing.B) {
	in := benchInput(200)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := ParseFast(strings.NewReader(in)); err != nil {
			b.Fatal(err)
		}
	}
}
