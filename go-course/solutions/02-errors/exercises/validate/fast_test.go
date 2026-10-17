package validate

import (
	"strings"
	"testing"
)

var fastCases = []Signup{
	valid,
	{},
	{Name: "A", Email: "bad", Age: 30},
	{Name: strings.Repeat("é", MaxNameLen+1), Email: "a@b.c", Age: 30},
	{Name: strings.Repeat("é", MaxNameLen), Email: "a@b.c", Age: 30},
	{Name: "A", Email: "a@b.c", Age: 3},
	{Name: " ", Email: "@x.y", Age: 999},
}

func TestValidateFastMatchesValidate(t *testing.T) {
	for _, in := range fastCases {
		want := Validate(in)
		got := ValidateFast(in)
		if (got == nil) != (want == nil) {
			t.Fatalf("ValidateFast(%+v) = %v, want %v", in, got, want)
		}
		if got != nil && got.Error() != want.Error() {
			t.Errorf("ValidateFast(%+v) = %q, want %q", in, got, want)
		}
	}
}

var sink error

func BenchmarkValidateValid(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = Validate(valid)
	}
}

func BenchmarkValidateFastValid(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = ValidateFast(valid)
	}
}

var invalid = Signup{Name: "", Email: "nope", Age: 30}

func BenchmarkValidateInvalid(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = Validate(invalid)
	}
}

func BenchmarkValidateFastInvalid(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = ValidateFast(invalid)
	}
}
