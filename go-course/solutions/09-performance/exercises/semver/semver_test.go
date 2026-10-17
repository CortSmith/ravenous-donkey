package semver

import (
	"errors"
	"testing"
)

func TestParseValid(t *testing.T) {
	tests := []struct {
		in   string
		want Version
	}{
		{"0.0.0", Version{0, 0, 0}},
		{"1.2.3", Version{1, 2, 3}},
		{"v1.2.3", Version{1, 2, 3}},
		{"v0.10.0", Version{0, 10, 0}},
		{"10.20.30", Version{10, 20, 30}},
		{"1.0.1234567", Version{1, 0, 1234567}},
		{"9223372036854775807.0.0", Version{9223372036854775807, 0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) returned error %v, want %+v", tt.in, err, tt.want)
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
		"v",
		"1",
		"1.2",
		"1.2.",
		".1.2",
		"1..2",
		"1.2.3.4",
		"V1.2.3",     // only lower-case v
		"vv1.2.3",    // at most one v
		"01.2.3",     // leading zero
		"1.02.3",     // leading zero
		"1.2.03",     // leading zero
		"-1.2.3",     // sign
		"+1.2.3",     // sign (strconv.Atoi alone would accept this)
		"1.+2.3",     // sign
		"1.2.3-rc1",  // pre-release not supported
		"1.2.3+meta", // build metadata not supported
		" 1.2.3",     // no surrounding space
		"1.2.3\n",    // no trailing newline
		"1.2.x",
		"a.b.c",
		"１.2.3",                   // full-width digit
		"9223372036854775808.0.0", // overflows int64
		"1.2.99999999999999999999",
	}
	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			got, err := Parse(in)
			if err == nil {
				t.Fatalf("Parse(%q) = %+v, nil; want an error", in, got)
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("Parse(%q) error = %v, want it to wrap ErrInvalid", in, err)
			}
			if got != (Version{}) {
				t.Errorf("Parse(%q) returned %+v with an error, want the zero Version", in, got)
			}
		})
	}
}

func TestParseAllocs(t *testing.T) {
	inputs := []string{"1.2.3", "v10.20.30", "0.0.0", "v123456.7.89"}
	allocs := testing.AllocsPerRun(200, func() {
		for _, in := range inputs {
			if _, err := Parse(in); err != nil {
				t.Fatalf("Parse(%q) failed: %v", in, err)
			}
		}
	})
	if allocs != 0 {
		t.Errorf("Parse of 4 valid versions allocates %.0f times, want 0.\n"+
			"Hint: a regexp compiled per call is very expensive, and even a precompiled "+
			"FindStringSubmatch allocates its result slice. strings.Cut and strconv.Atoi "+
			"do not allocate on success.", allocs)
	}
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		Parse("v123.45.6789")
	}
}
