package describe

import (
	"errors"
	"testing"
)

var sample = []any{
	nil, true, 42, int8(-3), int16(9), int32(7), int64(-8), uint(1), uint8(2), uint16(3), uint32(4), uint64(5),
	2.5, float32(0.1), "hello", []byte("abc"), errors.New("e"), celsius(20), both{},
	[]any{1, "x", []any{nil}}, map[string]any{"b": 1, "a": []any{2.0}}, point{}, []int{1},
}

func TestDescribeFastMatches(t *testing.T) {
	for _, v := range sample {
		if got, want := DescribeFast(v), Describe(v); got != want {
			t.Errorf("DescribeFast(%#v) = %q, Describe = %q", v, got, want)
		}
	}
	if got, want := DescribeFast(sample), Describe(sample); got != want {
		t.Errorf("DescribeFast(sample) = %q, want %q", got, want)
	}
}

func BenchmarkDescribe(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = Describe(sample)
	}
}

func BenchmarkDescribeFast(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = DescribeFast(sample)
	}
}

func BenchmarkAppendDescribeReuse(b *testing.B) {
	buf := make([]byte, 0, 1024)
	b.ReportAllocs()
	for b.Loop() {
		buf = AppendDescribe(buf[:0], sample)
	}
}
