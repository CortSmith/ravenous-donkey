package appendints

import "testing"

func TestAppendIntsFastMatches(t *testing.T) {
	nums := testNums(500)
	want := string(AppendInts([]byte("x="), nums, ' '))
	if got := string(AppendIntsFast([]byte("x="), nums, ' ')); got != want {
		t.Errorf("AppendIntsFast differs from AppendInts")
	}
	if got := AppendIntsFast(nil, nil, ','); got != nil {
		t.Errorf("AppendIntsFast(nil, nil) = %q, want nil", got)
	}
}

// BenchmarkAppendIntsNilDst compares the two when the caller does NOT reuse a
// buffer, which is where up-front growth pays off.
func BenchmarkAppendIntsNilDst(b *testing.B) {
	nums := testNums(200)
	b.Run("AppendInts", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			AppendInts(nil, nums, ',')
		}
	})
	b.Run("AppendIntsFast", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			AppendIntsFast(nil, nums, ',')
		}
	})
}
