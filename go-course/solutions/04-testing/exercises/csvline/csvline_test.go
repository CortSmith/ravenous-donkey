package csvline

import (
	"errors"
	"slices"
	"testing"
	"unicode/utf8"
)

func TestJoin(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   string
	}{
		{"nil", nil, ""},
		{"one empty field", []string{""}, ""},
		{"plain", []string{"a", "b", "c"}, "a,b,c"},
		{"empty fields", []string{"", "x", ""}, ",x,"},
		{"comma", []string{"a,b", "c"}, `"a,b",c`},
		{"newline", []string{"two\nlines"}, "\"two\nlines\""},
		{"carriage return", []string{"a\rb"}, "\"a\rb\""},
		{"quote inside", []string{`say "hi"`}, `"say ""hi"""`},
		// Found by FuzzRoundTrip: a leading quote must be quoted, or Split
		// reads the field as a quoted one.
		{"leading quote", []string{`"x`, "y"}, `"""x",y`},
		{"just a quote", []string{`"`}, `""""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Join(tt.fields); got != tt.want {
				t.Errorf("Join(%q) = %q, want %q", tt.fields, got, tt.want)
			}
		})
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    []string
		wantErr error
	}{
		{name: "empty line", line: "", want: []string{""}},
		{name: "plain", line: "a,b,c", want: []string{"a", "b", "c"}},
		{name: "trailing comma", line: "a,", want: []string{"a", ""}},
		{name: "only commas", line: ",,", want: []string{"", "", ""}},
		{name: "quoted comma", line: `"a,b",c`, want: []string{"a,b", "c"}},
		{name: "escaped quotes", line: `"say ""hi""",x`, want: []string{`say "hi"`, "x"}},
		{name: "empty quoted field", line: `"",x`, want: []string{"", "x"}},
		{name: "quoted last field", line: `a,"b"`, want: []string{"a", "b"}},
		{name: "quote mid-field is literal", line: `a"b,c`, want: []string{`a"b`, "c"}},
		{name: "unterminated", line: `"abc`, wantErr: ErrUnterminated},
		{name: "unterminated after escape", line: `"a""`, wantErr: ErrUnterminated},
		{name: "junk after closing quote", line: `"a"bcdefghijk,x`, wantErr: ErrSyntax},
		// Found by FuzzSplit: the error message's context snippet sliced
		// past the end of a short line and panicked.
		{name: "short junk after closing quote", line: `"a"b`, wantErr: ErrSyntax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Split(tt.line)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Split(%q) error = %v, want %v", tt.line, err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Split(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

// FuzzRoundTrip checks the package's central promise: Split undoes Join.
// The seeds are ordinary inputs; the fuzzer mutates them to find the bug.
//
//	go test ./modules/04-testing/exercises/csvline/ -run='^$' -fuzz=FuzzRoundTrip -fuzztime=20s
func FuzzRoundTrip(f *testing.F) {
	f.Add("a", "b", "c")
	f.Add("hello, world", "", "x")
	f.Add("two\nlines", "tab\there", "ünïcödé")
	f.Fuzz(func(t *testing.T, a, b, c string) {
		in := []string{a, b, c}
		line := Join(in)
		got, err := Split(line)
		if err != nil {
			t.Fatalf("Split(Join(%q)) = %q: error %v", in, line, err)
		}
		if !slices.Equal(got, in) {
			t.Fatalf("Split(Join(%q)) = %q (line %q)", in, got, line)
		}
	})
}

// FuzzSplit feeds Split arbitrary lines. Split may reject them, but it must
// never panic, and whatever it accepts must survive a Join/Split round trip.
//
//	go test ./modules/04-testing/exercises/csvline/ -run='^$' -fuzz=FuzzSplit -fuzztime=20s
func FuzzSplit(f *testing.F) {
	for _, seed := range []string{"", "a,b,c", `"a,b",c`, `x,"say ""hi""",z`, ",,", `"abc`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, line string) {
		fields, err := Split(line)
		if err != nil {
			if !errors.Is(err, ErrUnterminated) && !errors.Is(err, ErrSyntax) {
				t.Fatalf("Split(%q) returned unexpected error kind: %v", line, err)
			}
			return
		}
		if len(fields) == 0 {
			t.Fatalf("Split(%q) returned no fields and no error", line)
		}
		if utf8.ValidString(line) {
			for _, f := range fields {
				if !utf8.ValidString(f) {
					t.Fatalf("Split(%q) produced invalid UTF-8 field %q", line, f)
				}
			}
		}
		again, err := Split(Join(fields))
		if err != nil || !slices.Equal(again, fields) {
			t.Fatalf("Split(Join(%q)) = %q, %v", fields, again, err)
		}
	})
}
