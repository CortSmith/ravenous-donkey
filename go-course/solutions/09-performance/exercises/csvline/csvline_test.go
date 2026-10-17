package csvline

import (
	"encoding/csv"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestLine(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   string
	}{
		{"nil", nil, ""},
		{"empty slice", []string{}, ""},
		{"one field", []string{"a"}, "a"},
		{"one empty field", []string{""}, ""},
		{"two empty fields", []string{"", ""}, ","},
		{"plain", []string{"id", "name", "age"}, "id,name,age"},
		{"comma", []string{"a,b", "c"}, `"a,b",c`},
		{"quote", []string{`say "hi"`}, `"say ""hi"""`},
		{"only quote", []string{`"`}, `""""`},
		{"newline", []string{"line1\nline2", "x"}, "\"line1\nline2\",x"},
		{"carriage return", []string{"a\rb"}, "\"a\rb\""},
		{"spaces are not quoted", []string{" a ", "b c"}, " a ,b c"},
		{"unicode", []string{"héllo", "世界,!"}, `héllo,"世界,!"`},
		{"mixed", []string{"", `a"b`, "c,d", "e"}, `,"a""b","c,d",e`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Line(tt.fields); got != tt.want {
				t.Errorf("Line(%q) = %q, want %q", tt.fields, got, tt.want)
			}
		})
	}
}

// TestLineRoundTrip checks Line against encoding/csv as an oracle: whatever
// Line writes, csv.Reader must read back as the original fields.
func TestLineRoundTrip(t *testing.T) {
	pieces := []string{"a", "bc", ",", `"`, "\n", " ", "x y", "", "é"}
	for i := range 500 {
		n := 1 + i%7
		fields := make([]string, n)
		for j := range fields {
			var sb strings.Builder
			for k := range (i + j) % 4 {
				sb.WriteString(pieces[(i*7+j*3+k)%len(pieces)])
			}
			fields[j] = sb.String()
		}
		line := Line(fields)
		if line == "" {
			continue // csv.Reader cannot represent a record that is a single empty field
		}
		r := csv.NewReader(strings.NewReader(line + "\n"))
		r.FieldsPerRecord = -1
		got, err := r.Read()
		if err != nil {
			t.Fatalf("csv.Reader could not read Line(%q) = %q: %v", fields, line, err)
		}
		if !slices.Equal(got, fields) {
			t.Fatalf("round trip of %q: Line = %q, read back %q", fields, line, got)
		}
	}
}

func TestLineAllocs(t *testing.T) {
	fields := make([]string, 40)
	for i := range fields {
		switch i % 4 {
		case 0:
			fields[i] = fmt.Sprintf("field%d", i)
		case 1:
			fields[i] = fmt.Sprintf("has,comma%d", i)
		case 2:
			fields[i] = fmt.Sprintf(`quote"%d"here`, i)
		default:
			fields[i] = ""
		}
	}
	want := Line(fields) // warm up, and check the result is stable
	allocs := testing.AllocsPerRun(100, func() {
		if got := Line(fields); len(got) != len(want) {
			t.Fatalf("Line returned a different length on repeat call")
		}
	})
	const maxAllocs = 1
	if allocs > maxAllocs {
		t.Errorf("Line(40 fields) allocates %.0f times per call, want <= %d.\n"+
			"Hint: s += x copies the whole string every time. Use a strings.Builder "+
			"and Grow it to the exact final size before writing.", allocs, maxAllocs)
	}
}

func BenchmarkLine(b *testing.B) {
	fields := make([]string, 40)
	for i := range fields {
		fields[i] = fmt.Sprintf("value,%d", i)
	}
	b.ReportAllocs()
	for b.Loop() {
		Line(fields)
	}
}
