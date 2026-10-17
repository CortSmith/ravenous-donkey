package csvjson

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestAppendJSONStringMatchesEncodingJSON(t *testing.T) {
	inputs := []string{
		"", "plain", `quote " and \ backslash`, "ctl \x00\x01\x1f\x7f", "\n\r\t\b\f",
		"<html> & </html>", "日本語 ✓", "line\u2028sep\u2029para", "bad \xff\xfe utf8", "\xe6\x97",
	}
	for _, in := range inputs {
		want, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		got := appendJSONString(nil, in)
		if string(got) != string(want) {
			t.Errorf("appendJSONString(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestConvertFastMatchesConvert(t *testing.T) {
	inputs := []string{
		"a,b\n",
		"name,age\nAnn,31\nBob,42\n",
		"q,r\n\"a, b\",\"say \"\"hi\"\"\"\n\"multi\nline\",x\n",
		"\uFEFFk,<v>\n\"x\ty\",&\n",
	}
	for _, in := range inputs {
		var want, got strings.Builder
		if err := Convert(&want, strings.NewReader(in)); err != nil {
			t.Fatal(err)
		}
		if err := ConvertFast(&got, strings.NewReader(in)); err != nil {
			t.Fatalf("ConvertFast(%q) error = %v", in, err)
		}
		w, g := decodeLines(t, want.String()), decodeLines(t, got.String())
		if !reflect.DeepEqual(g, w) {
			t.Errorf("ConvertFast(%q) = %v, want %v", in, g, w)
		}
	}
	for _, in := range []string{"", "a,a\n", "a\n1,2\n"} {
		errA := Convert(new(strings.Builder), strings.NewReader(in))
		errB := ConvertFast(new(strings.Builder), strings.NewReader(in))
		if (errA == nil) != (errB == nil) || errors.Unwrap(errA) != nil && errA.Error() != errB.Error() {
			t.Errorf("errors differ for %q: Convert=%v ConvertFast=%v", in, errA, errB)
		}
	}
}

func benchCSV() string {
	var sb strings.Builder
	sb.WriteString("id,name,email,city,note\n")
	for range 10000 {
		sb.WriteString(`42,Ada Lovelace,ada@example.com,London,"First programmer, arguably"` + "\n")
	}
	return sb.String()
}

var benchInput = benchCSV()

func BenchmarkConvert(b *testing.B) {
	b.SetBytes(int64(len(benchInput)))
	b.ReportAllocs()
	for b.Loop() {
		if err := Convert(discard{}, strings.NewReader(benchInput)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConvertFast(b *testing.B) {
	b.SetBytes(int64(len(benchInput)))
	b.ReportAllocs()
	for b.Loop() {
		if err := ConvertFast(discard{}, strings.NewReader(benchInput)); err != nil {
			b.Fatal(err)
		}
	}
}

// discard is io.Discard without the ReaderFrom fast path, so both benchmarks
// do the same amount of writing work.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
