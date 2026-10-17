package appendints

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestAppendInts(t *testing.T) {
	tests := []struct {
		name string
		dst  string
		nums []int
		sep  byte
		want string
	}{
		{"nil nums", "", nil, ',', ""},
		{"empty nums keeps prefix", "keep", []int{}, ',', "keep"},
		{"zero", "", []int{0}, ',', "0"},
		{"single", "", []int{42}, ',', "42"},
		{"several", "", []int{1, 22, 333}, ',', "1,22,333"},
		{"negative", "", []int{-1, -250, 7}, ' ', "-1 -250 7"},
		{"tab separator", "", []int{10, 20}, '\t', "10\t20"},
		{"prefix preserved", "ids=", []int{5, 6}, ';', "ids=5;6"},
		{"min and max", "", []int{math.MinInt64, math.MaxInt64}, ',', "-9223372036854775808,9223372036854775807"},
		{"small ints that fmt can box for free", "", []int{0, 1, 255, 256}, ',', "0,1,255,256"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst []byte
			if tt.dst != "" {
				dst = []byte(tt.dst)
			}
			got := string(AppendInts(dst, tt.nums, tt.sep))
			if got != tt.want {
				t.Errorf("AppendInts(%q, %v, %q) = %q, want %q", tt.dst, tt.nums, tt.sep, got, tt.want)
			}
		})
	}
}

func TestAppendIntsMatchesStrconv(t *testing.T) {
	nums := testNums(1000)
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa(n)
	}
	want := strings.Join(parts, ",")
	if got := string(AppendInts(nil, nums, ',')); got != want {
		t.Errorf("AppendInts(nil, 1000 nums, ',') mismatch:\n got %.80q...\nwant %.80q...", got, want)
	}
}

func TestAppendIntsDoesNotClobberSpareCapacity(t *testing.T) {
	// dst has spare capacity; AppendInts must write after len(dst), never before.
	buf := make([]byte, 3, 64)
	copy(buf, "ab:")
	out := AppendInts(buf, []int{1, 2}, ',')
	if string(out) != "ab:1,2" {
		t.Errorf("AppendInts(%q, [1 2], ',') = %q, want %q", "ab:", out, "ab:1,2")
	}
	if string(buf) != "ab:" {
		t.Errorf("AppendInts modified the visible part of dst: now %q, want %q", buf, "ab:")
	}
}

func TestAppendIntsAllocs(t *testing.T) {
	nums := testNums(200)
	dst := make([]byte, 0, 8192) // plenty of room: AppendInts should never need to grow it
	want := string(AppendInts(nil, nums, ','))
	allocs := testing.AllocsPerRun(100, func() {
		dst = AppendInts(dst[:0], nums, ',')
	})
	if string(dst) != want {
		t.Fatalf("AppendInts output changed between calls")
	}
	if allocs != 0 {
		t.Errorf("AppendInts(dst with spare capacity, 200 nums) allocates %.0f times per call, want 0.\n"+
			"Hint: fmt.Sprintf builds a new string (and boxes its argument). "+
			"strconv has a function that appends digits directly to a []byte.", allocs)
	}
}

// testNums returns n deterministic integers with a mix of signs and magnitudes.
func testNums(n int) []int {
	nums := make([]int, n)
	x := 1
	for i := range nums {
		x = x*1103515245 + 12345
		v := x % 10_000_000
		if i%3 == 0 {
			v = -v
		}
		nums[i] = v
	}
	return nums
}

func BenchmarkAppendInts(b *testing.B) {
	nums := testNums(200)
	dst := make([]byte, 0, 8192)
	b.ReportAllocs()
	for b.Loop() {
		dst = AppendInts(dst[:0], nums, ',')
	}
}
