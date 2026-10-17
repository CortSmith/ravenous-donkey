package semver_test

import (
	"testing"

	"gocourse/modules/04-testing/project/semver"
)

// Write comprehensive tests to expose the 4 bugs in semver.go and reach >=90% coverage.
//
// TODO: Write TestParse (table-driven, black-box):
//   - Valid versions: plain (1.0.0), with v prefix, with prerelease, with build
//
// TODO: Write TestParseInvalid (table-driven):
//   - Everything the spec forbids: go through semver.org rule by rule
//
// TODO: Write TestMustParse, TestString, TestIsPrerelease, TestCompare, TestCompareSpecOrder, TestSort
//
// TODO: Write FuzzParse and FuzzCompare with proper seeds
//
// TODO: Write BenchmarkParse and BenchmarkCompare

func TestParseSimple(t *testing.T) {
	_, err := semver.Parse("1.0.0")
	if err != nil {
		t.Errorf("Parse(\"1.0.0\") error = %v, want nil", err)
	}
}
