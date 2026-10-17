package csvline

import (
	"fmt"
	"testing"
)

func TestAppendLineMatchesLine(t *testing.T) {
	inputs := [][]string{
		nil, {""}, {"a"}, {"a,b", `c"d`, "e\nf", ""}, {`"`, `""`, "plain"},
	}
	for _, fields := range inputs {
		got := string(AppendLine([]byte("prefix:"), fields))
		if want := "prefix:" + Line(fields); got != want {
			t.Errorf("AppendLine(%q) = %q, want %q", fields, got, want)
		}
	}
}

func TestAppendLineAllocs(t *testing.T) {
	fields := []string{"a,b", `c"d`, "plain", "more"}
	buf := make([]byte, 0, 256)
	allocs := testing.AllocsPerRun(100, func() {
		buf = AppendLine(buf[:0], fields)
	})
	if allocs != 0 {
		t.Errorf("AppendLine with a reused buffer allocates %.0f times, want 0", allocs)
	}
}

func BenchmarkLineFast(b *testing.B) {
	fields := make([]string, 40)
	for i := range fields {
		fields[i] = fmt.Sprintf("value,%d", i)
	}
	b.Run("Line", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			Line(fields)
		}
	})
	b.Run("AppendLine", func(b *testing.B) {
		b.ReportAllocs()
		var buf []byte
		for b.Loop() {
			buf = AppendLine(buf[:0], fields)
		}
	})
}
