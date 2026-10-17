package csvjson

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// decodeLines parses NDJSON output into one map per line and fails the test
// if any line is not a JSON object of strings.
func decodeLines(t *testing.T, out string) []map[string]string {
	t.Helper()
	if out == "" {
		return nil
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("output does not end with a newline: %q", out)
	}
	var rows []map[string]string
	for i, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		var m map[string]string
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("output line %d is not a JSON object of strings: %v\nline: %s", i+1, err, line)
		}
		rows = append(rows, m)
	}
	return rows
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []map[string]string
	}{
		{
			name: "header only",
			in:   "a,b\n",
			want: nil,
		},
		{
			name: "simple",
			in:   "name,age\nAnn,31\nBob,42\n",
			want: []map[string]string{
				{"name": "Ann", "age": "31"},
				{"name": "Bob", "age": "42"},
			},
		},
		{
			name: "no trailing newline",
			in:   "x\n1",
			want: []map[string]string{{"x": "1"}},
		},
		{
			name: "crlf line endings",
			in:   "a,b\r\n1,2\r\n",
			want: []map[string]string{{"a": "1", "b": "2"}},
		},
		{
			name: "quoted fields with commas quotes and newlines",
			in:   "q,r\n\"a, b\",\"say \"\"hi\"\"\"\n\"multi\nline\",x\n",
			want: []map[string]string{
				{"q": "a, b", "r": `say "hi"`},
				{"q": "multi\nline", "r": "x"},
			},
		},
		{
			name: "json special characters",
			in:   "k\n\"back\\slash \"\"quote\"\" <tag> & tab\t\"\n",
			want: []map[string]string{{"k": "back\\slash \"quote\" <tag> & tab\t"}},
		},
		{
			name: "empty values and blank lines",
			in:   "a,b,c\n,,\n\n1,,3\n",
			want: []map[string]string{
				{"a": "", "b": "", "c": ""},
				{"a": "1", "b": "", "c": "3"},
			},
		},
		{
			name: "unicode",
			in:   "город,名前\nМосква,太郎\n",
			want: []map[string]string{{"город": "Москва", "名前": "太郎"}},
		},
		{
			name: "byte order mark is stripped",
			in:   "\uFEFFid,v\n7,x\n",
			want: []map[string]string{{"id": "7", "v": "x"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			if err := Convert(&out, strings.NewReader(tt.in)); err != nil {
				t.Fatalf("Convert(%q) error = %v", tt.in, err)
			}
			got := decodeLines(t, out.String())
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Convert(%q):\n got %v\nwant %v\nraw output: %q", tt.in, got, tt.want, out.String())
			}
		})
	}
}

func TestConvertErrors(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantIs  error // checked with errors.Is when non-nil
		wantMsg string
	}{
		{name: "empty input", in: "", wantIs: ErrNoHeader},
		{name: "only blank lines", in: "\n\n", wantIs: ErrNoHeader},
		{name: "empty column name", in: "a,,c\n1,2,3\n", wantIs: ErrBadHeader, wantMsg: "column 2"},
		{name: "duplicate column", in: "id,name,id\n1,2,3\n", wantIs: ErrBadHeader, wantMsg: `"id"`},
		{name: "too many fields", in: "a,b\n1,2\n1,2,3\n", wantIs: csv.ErrFieldCount},
		{name: "too few fields", in: "a,b\n1\n", wantIs: csv.ErrFieldCount},
		{name: "bare quote", in: "a\nx\"y\n", wantIs: csv.ErrBareQuote},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			err := Convert(&out, strings.NewReader(tt.in))
			if err == nil {
				t.Fatalf("Convert(%q) error = nil, want an error", tt.in)
			}
			if !errors.Is(err, tt.wantIs) {
				t.Errorf("Convert(%q) error = %v, want errors.Is(err, %v)", tt.in, err, tt.wantIs)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Convert(%q) error = %q, want it to mention %s", tt.in, err, tt.wantMsg)
			}
		})
	}
}

// TestParseErrorLine checks that the csv package's *ParseError (with its
// line number) survives wrapping, and that rows before the bad one were
// already streamed out.
func TestParseErrorLine(t *testing.T) {
	in := "a,b\n1,2\n3,4\n5,6,7\n8,9\n"
	var out strings.Builder
	err := Convert(&out, strings.NewReader(in))
	var pe *csv.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("Convert error = %v, want it to wrap a *csv.ParseError", err)
	}
	if pe.Line != 4 {
		t.Errorf("ParseError.Line = %d, want 4", pe.Line)
	}
	if got := len(decodeLines(t, out.String())); got != 2 {
		t.Errorf("rows written before the error = %d, want 2 (Convert should stream)", got)
	}
}

type failWriter struct{ err error }

func (w failWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteErrorPropagates(t *testing.T) {
	boom := errors.New("disk full")
	err := Convert(failWriter{boom}, strings.NewReader("a\n1\n"))
	if !errors.Is(err, boom) {
		t.Errorf("Convert to failing writer: error = %v, want errors.Is(err, %v)", err, boom)
	}
}

// TestTestdata converts testdata/people.csv and compares it, row by row and
// key by key, with testdata/people.jsonl. Key order inside an object does
// not matter; row order does.
func TestTestdata(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "people.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var out strings.Builder
	if err := Convert(&out, f); err != nil {
		t.Fatalf("Convert(people.csv) error = %v", err)
	}
	got := decodeLines(t, out.String())

	wf, err := os.Open(filepath.Join("testdata", "people.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer wf.Close()
	var want []map[string]string
	sc := bufio.NewScanner(wf)
	for sc.Scan() {
		var m map[string]string
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("bad golden line %q: %v", sc.Text(), err)
		}
		want = append(want, m)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}

	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("row %d:\n got %v\nwant %v", i+1, got[i], want[i])
		}
	}
}
