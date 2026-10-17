package semver_test // black-box: only the exported API, like a real user

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"gocourse/solutions/04-testing/project/semver"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want semver.Version
	}{
		{"0.0.0", semver.Version{}},
		{"1.2.3", semver.Version{Major: 1, Minor: 2, Patch: 3}},
		{"v1.2.3", semver.Version{Major: 1, Minor: 2, Patch: 3}},
		{"10.20.30", semver.Version{Major: 10, Minor: 20, Patch: 30}},
		{"1.0.0-alpha", semver.Version{Major: 1, Prerelease: "alpha"}},
		{"1.0.0-alpha.1", semver.Version{Major: 1, Prerelease: "alpha.1"}},
		{"1.0.0-0.3.7", semver.Version{Major: 1, Prerelease: "0.3.7"}},
		{"1.0.0-x-y-z.--", semver.Version{Major: 1, Prerelease: "x-y-z.--"}},
		{"1.0.0+20130313144700", semver.Version{Major: 1, Build: "20130313144700"}},
		{"1.0.0-beta+exp.sha.5114f85", semver.Version{Major: 1, Prerelease: "beta", Build: "exp.sha.5114f85"}},
		{"1.0.0+21AF26D3---117B344092BD", semver.Version{Major: 1, Build: "21AF26D3---117B344092BD"}},
		{"1.0.0+001", semver.Version{Major: 1, Build: "001"}}, // leading zeros OK in build
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := semver.Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []string{
		"",
		"1",
		"1.2",
		"1.2.3.4",
		"1..3",
		"a.b.c",
		"1.2.c",
		"-1.2.3",
		"+1.2.3",
		"1.+2.3",
		" 1.2.3",
		"vv1.2.3",
		"V1.2.3",
		"01.2.3", // leading zeros are forbidden in MAJOR/MINOR/PATCH
		"1.02.3",
		"1.2.00",
		"1.2.3-",     // empty pre-release
		"1.2.3-a..b", // empty identifier
		"1.2.3-a.",
		"1.2.3+",
		"1.2.3+a..b",
		"1.2.3-01", // numeric pre-release identifier with leading zero
		"1.2.3-a_b",
		"1.2.3+a b",
		"1.2.3-é",
		"99999999999999999999.0.0", // overflows int
	}
	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			v, err := semver.Parse(in)
			if !errors.Is(err, semver.ErrInvalid) {
				t.Errorf("Parse(%q) = %+v, %v; want error wrapping ErrInvalid", in, v, err)
			}
		})
	}
}

func TestMustParse(t *testing.T) {
	if got := semver.MustParse("1.2.3"); got != (semver.Version{Major: 1, Minor: 2, Patch: 3}) {
		t.Errorf("MustParse(\"1.2.3\") = %+v", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("MustParse(\"bad\") did not panic")
		}
	}()
	semver.MustParse("bad")
}

func TestString(t *testing.T) {
	tests := []struct {
		v    semver.Version
		want string
	}{
		{semver.Version{}, "0.0.0"},
		{semver.Version{Major: 1, Minor: 2, Patch: 3}, "1.2.3"},
		{semver.Version{Major: 1, Prerelease: "rc.1"}, "1.0.0-rc.1"},
		{semver.Version{Major: 1, Build: "b7"}, "1.0.0+b7"},
		{semver.Version{Major: 1, Prerelease: "rc.1", Build: "b7"}, "1.0.0-rc.1+b7"},
	}
	for _, tt := range tests {
		if got := tt.v.String(); got != tt.want {
			t.Errorf("%+v.String() = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestIsPrerelease(t *testing.T) {
	if semver.MustParse("1.0.0").IsPrerelease() {
		t.Error("1.0.0 reported as a pre-release")
	}
	if !semver.MustParse("1.0.0-rc.1").IsPrerelease() {
		t.Error("1.0.0-rc.1 not reported as a pre-release")
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "2.0.0", -1},
		{"1.2.3", "1.3.0", -1},
		{"1.2.3", "1.2.4", -1},
		{"2.0.0", "10.0.0", -1}, // numeric, not string, order
		{"1.0.0-rc.1", "1.0.0", -1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.2", "1.0.0-alpha.10", -1},
		{"1.0.0-1", "1.0.0-alpha", -1}, // numeric < non-numeric
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-B", "1.0.0-a", -1}, // ASCII order: uppercase first
		{"1.0.0+a", "1.0.0+b", 0},  // build ignored
		{"1.0.0-rc.1+a", "1.0.0-rc.1+b", 0},
		{"1.0.0-99999999999999999999", "1.0.0-100000000000000000000", -1}, // no overflow
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			a, b := semver.MustParse(tt.a), semver.MustParse(tt.b)
			if got := semver.Compare(a, b); got != tt.want {
				t.Errorf("Compare(%s, %s) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
			if got := semver.Compare(b, a); got != -tt.want {
				t.Errorf("Compare(%s, %s) = %d, want %d", tt.b, tt.a, got, -tt.want)
			}
		})
	}
}

// specOrder is the precedence example from semver.org §11, ascending.
var specOrder = []string{
	"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
	"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0",
}

func TestCompareSpecOrder(t *testing.T) {
	for i := range specOrder {
		for j := range specOrder {
			a, b := semver.MustParse(specOrder[i]), semver.MustParse(specOrder[j])
			want := 0
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			if got := semver.Compare(a, b); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", a, b, got, want)
			}
		}
	}
}

func TestSort(t *testing.T) {
	in := []string{"1.0.0", "1.0.0-beta.11", "1.0.0-alpha", "0.9.0", "1.0.0-beta.2", "1.0.0+z"}
	vs := make([]semver.Version, len(in))
	for i, s := range in {
		vs[i] = semver.MustParse(s)
	}
	semver.Sort(vs)
	var got []string
	for _, v := range vs {
		got = append(got, v.String())
	}
	// 1.0.0 and 1.0.0+z have equal precedence and keep their input order.
	want := []string{"0.9.0", "1.0.0-alpha", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0", "1.0.0+z"}
	if !slices.Equal(got, want) {
		t.Errorf("Sort(%v) = %v, want %v", in, got, want)
	}
}

// FuzzParse checks properties that must hold for every input:
// Parse never panics; anything it accepts round-trips through String; and
// without a "v" prefix, String reproduces the input exactly.
//
//	go test ./modules/04-testing/project/semver/ -run='^$' -fuzz=FuzzParse -fuzztime=30s
func FuzzParse(f *testing.F) {
	for _, s := range []string{"1.2.3", "v0.1.0", "1.0.0-alpha.1", "2.0.0-rc.1+build.5", "1.0.0+20130313"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v, err := semver.Parse(s)
		if err != nil {
			if !errors.Is(err, semver.ErrInvalid) {
				t.Fatalf("Parse(%q) error %v does not wrap ErrInvalid", s, err)
			}
			return
		}
		if v.Major < 0 || v.Minor < 0 || v.Patch < 0 {
			t.Fatalf("Parse(%q) = %+v: negative component", s, v)
		}
		if want := strings.TrimPrefix(s, "v"); v.String() != want {
			t.Fatalf("Parse(%q).String() = %q, want %q", s, v.String(), want)
		}
		again, err := semver.Parse(v.String())
		if err != nil || again != v {
			t.Fatalf("Parse(%q) = %+v, but Parse(String()) = %+v, %v", s, v, again, err)
		}
		if c := semver.Compare(v, v); c != 0 {
			t.Fatalf("Compare(%s, itself) = %d", v, c)
		}
	})
}

// FuzzCompare checks that Compare is antisymmetric for any two valid
// versions.
func FuzzCompare(f *testing.F) {
	f.Add("1.0.0", "1.0.0-rc.1")
	f.Add("1.0.0-alpha.2", "1.0.0-alpha.10")
	f.Fuzz(func(t *testing.T, x, y string) {
		a, errA := semver.Parse(x)
		b, errB := semver.Parse(y)
		if errA != nil || errB != nil {
			t.Skip()
		}
		if ab, ba := semver.Compare(a, b), semver.Compare(b, a); ab != -ba {
			t.Fatalf("Compare(%s, %s) = %d but Compare(%s, %s) = %d", a, b, ab, b, a, ba)
		}
	})
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		semver.Parse("v1.22.333-rc.1.alpha+build.2025")
	}
}

func BenchmarkCompare(b *testing.B) {
	x := semver.MustParse("1.2.3-alpha.beta.10")
	y := semver.MustParse("1.2.3-alpha.beta.9")
	b.ReportAllocs()
	for b.Loop() {
		semver.Compare(x, y)
	}
}

func ExampleCompare() {
	a := semver.MustParse("1.0.0-rc.1")
	b := semver.MustParse("1.0.0")
	fmt.Println(semver.Compare(a, b))
	// Output: -1
}

func ExampleSort() {
	vs := []semver.Version{
		semver.MustParse("1.10.0"),
		semver.MustParse("1.9.0"),
		semver.MustParse("1.10.0-beta"),
	}
	semver.Sort(vs)
	fmt.Println(vs)
	// Output: [1.9.0 1.10.0-beta 1.10.0]
}
