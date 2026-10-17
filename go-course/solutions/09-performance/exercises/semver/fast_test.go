package semver

import (
	"regexp"
	"strconv"
	"testing"
)

// FuzzParseFastMatchesParse checks the two implementations agree on every input.
// Run it with: go test -fuzz=FuzzParseFastMatchesParse ./solutions/09-performance/exercises/semver/
func FuzzParseFastMatchesParse(f *testing.F) {
	for _, s := range []string{"1.2.3", "v0.0.0", "01.2.3", "1.2.3.4", "9223372036854775808.0.0", "v", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v1, err1 := Parse(s)
		v2, err2 := ParseFast(s)
		if v1 != v2 || (err1 == nil) != (err2 == nil) {
			t.Errorf("Parse(%q) = %+v, %v; ParseFast = %+v, %v", s, v1, err1, v2, err2)
		}
	})
}

// semverRE is the "hoist the regexp" half-measure, kept here only so the
// benchmark can show why it is not enough.
var semverRE = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func parseHoistedRegexp(s string) (Version, error) {
	m := semverRE.FindStringSubmatch(s)
	if m == nil {
		return Version{}, invalid(s)
	}
	var nums [3]int
	for i, part := range m[1:] {
		n, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, invalid(s)
		}
		nums[i] = n
	}
	return Version{nums[0], nums[1], nums[2]}, nil
}

func BenchmarkParseVariants(b *testing.B) {
	const in = "v123.45.6789"
	b.Run("HoistedRegexp", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			parseHoistedRegexp(in)
		}
	})
	b.Run("Parse", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			Parse(in)
		}
	})
	b.Run("ParseFast", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			ParseFast(in)
		}
	})
}
