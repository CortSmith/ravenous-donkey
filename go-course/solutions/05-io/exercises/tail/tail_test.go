package tail

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

func TestTail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want []string
	}{
		{"empty input", "", 3, []string{}},
		{"n zero", "a\nb\n", 0, []string{}},
		{"n negative", "a\nb\n", -1, []string{}},
		{"fewer lines than n", "a\nb\n", 5, []string{"a", "b"}},
		{"exactly n lines", "a\nb\nc\n", 3, []string{"a", "b", "c"}},
		{"more lines than n", "1\n2\n3\n4\n5\n", 2, []string{"4", "5"}},
		{"n is one", "1\n2\n3\n", 1, []string{"3"}},
		{"wraps several times", "1\n2\n3\n4\n5\n6\n7\n", 3, []string{"5", "6", "7"}},
		{"no trailing newline", "1\n2\n3", 2, []string{"2", "3"}},
		{"crlf stripped", "a\r\nb\r\nc\r\n", 2, []string{"b", "c"}},
		{"blank lines are lines", "a\n\n\nb\n\n", 3, []string{"", "b", ""}},
		{"only newlines", "\n\n\n", 2, []string{"", ""}},
		{"unicode", "один\nдва\nтри\n", 2, []string{"два", "три"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Tail(strings.NewReader(tt.in), tt.n)
			if err != nil {
				t.Fatalf("Tail(%q, %d) error = %v", tt.in, tt.n, err)
			}
			if got == nil {
				t.Fatalf("Tail(%q, %d) = nil, want a non-nil (possibly empty) slice", tt.in, tt.n)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tail(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
			}
		})
	}
}

// TestNZeroDoesNotRead checks that Tail returns immediately for n <= 0.
func TestNZeroDoesNotRead(t *testing.T) {
	r := iotest.ErrReader(errors.New("must not be read"))
	got, err := Tail(r, 0)
	if err != nil || len(got) != 0 {
		t.Errorf("Tail(errReader, 0) = %q, %v; want [], nil", got, err)
	}
}

func TestManyLines(t *testing.T) {
	var sb strings.Builder
	const total = 100_000
	for i := range total {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	for _, n := range []int{1, 7, 1000, total - 1, total, total + 1} {
		got, err := Tail(strings.NewReader(sb.String()), n)
		if err != nil {
			t.Fatalf("Tail(..., %d) error = %v", n, err)
		}
		wantLen := min(n, total)
		if len(got) != wantLen {
			t.Fatalf("Tail(..., %d) returned %d lines, want %d", n, len(got), wantLen)
		}
		for i, line := range got {
			want := fmt.Sprintf("line %d", total-wantLen+i)
			if line != want {
				t.Fatalf("Tail(..., %d)[%d] = %q, want %q", n, i, line, want)
			}
		}
	}
}

func TestHugeNDoesNotPreallocate(t *testing.T) {
	// A naive make([]string, n) would try to allocate 16 EiB here.
	got, err := Tail(strings.NewReader("a\nb\n"), int(^uint(0)>>1))
	if err != nil {
		t.Fatalf("Tail error = %v", err)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Tail = %q, want %q", got, want)
	}
}

func TestLongLines(t *testing.T) {
	ok := strings.Repeat("x", 200_000) // longer than bufio's default 64 KiB limit
	got, err := Tail(strings.NewReader("a\n"+ok+"\nb\n"), 2)
	if err != nil {
		t.Fatalf("Tail with a 200 KB line: error = %v (raise the Scanner's buffer limit)", err)
	}
	if len(got) != 2 || got[0] != ok || got[1] != "b" {
		t.Errorf("Tail with a 200 KB line returned wrong lines (len %d)", len(got))
	}

	tooLong := strings.Repeat("y", MaxLineLength+1)
	got, err = Tail(strings.NewReader("a\n"+tooLong+"\n"), 2)
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("Tail with a line over MaxLineLength: error = %v, want errors.Is(err, bufio.ErrTooLong)", err)
	}
	if got != nil {
		t.Errorf("Tail on error returned %d lines, want nil", len(got))
	}
}

func TestPropagatesErrors(t *testing.T) {
	boom := errors.New("network went away")
	r := io.MultiReader(strings.NewReader("a\nb\n"), iotest.ErrReader(boom))
	got, err := Tail(r, 5)
	if !errors.Is(err, boom) {
		t.Errorf("Tail error = %v, want errors.Is(err, %v)", err, boom)
	}
	if got != nil {
		t.Errorf("Tail on error = %q, want nil", got)
	}
}

func TestOneByteReader(t *testing.T) {
	got, err := Tail(iotest.OneByteReader(strings.NewReader("a\r\nbb\r\nccc")), 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bb", "ccc"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Tail = %q, want %q", got, want)
	}
}

func TestTestdata(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "proverbs.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := Tail(f, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Design the architecture, name the components, document the details.",
		"Documentation is for users.",
		"Don't panic.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Tail(proverbs.txt, 3) =\n%q\nwant\n%q", got, want)
	}
}
