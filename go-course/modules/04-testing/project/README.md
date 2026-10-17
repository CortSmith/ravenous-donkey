# Project: Semantic Versioning Parser

Implement a semantic versioning parser and comparison tool per [semver.org spec v2.0.0](https://semver.org/spec/v2.0.0.html).

## Overview

Semantic versioning follows the format `MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]`:
- `1.0.0` — release version.
- `1.0.0-alpha` — pre-release version.
- `1.0.0-rc.1+build.5` — pre-release with build metadata.
- `v1.0.0` — optionally prefixed with "v".

**Rules:**
- MAJOR, MINOR, PATCH are non-negative integers without leading zeros.
- Pre-release and build are dot-separated identifiers (alphanumeric + hyphen). Pre-release identifiers that are numeric must not have leading zeros.
- Build metadata is ignored in version precedence (v1.0.0+a == v1.0.0+b).
- A pre-release version is lower precedence than the release (1.0.0-rc.1 < 1.0.0).
- Pre-release identifiers compare left-to-right: numeric identifiers numerically, others alphabetically, numeric < non-numeric.

## Starter Code

`semver.go` contains:
- **Parse(s string) (Version, error)**: parse a version string.
- **MustParse(s string) Version**: Parse that panics on error (for constants).
- **Version.String() string**: canonical form without "v".
- **Version.IsPrerelease() bool**: whether it has pre-release identifiers.
- **Compare(a, b Version) int**: returns -1, 0, or 1 for <, ==, >.
- **Sort(vs []Version)**: sort versions in ascending precedence.

The starter code **has 4 planted bugs** and no tests. Your job is to find
them the way you would in real life: by testing the code against the
specification, not by reading it line by line. The spec at
[semver.org](https://semver.org/spec/v2.0.0.html) is your oracle; §2, §9,
§10 and §11 contain every rule you need. One of the bugs makes `Parse`
panic on a certain input; a fuzz test will find it if your table doesn't.

## Requirements

### Parsing

- Accept optional "v" prefix (case-sensitive, only lowercase "v").
- Parse MAJOR.MINOR.PATCH (all required, integers without leading zeros).
- Optional "-PRERELEASE" and "+BUILD" suffixes.
- Validate identifiers (non-empty, alphanumeric + hyphen, numeric ones without leading zeros in prerelease).
- On failure, return the zero `Version` and an error that wraps `ErrInvalid` (check it with `errors.Is`).

### Comparison

Implement precedence rules from semver spec §11:
- Compare MAJOR, MINOR, PATCH numerically, in that order.
- A pre-release is lower than the release (1.0.0-rc < 1.0.0).
- Pre-release identifiers compare left-to-right.
- Numeric identifiers compare numerically (e.g., "2" < "10"), and must not overflow on very long ones.
- Non-numeric: ASCII order (e.g., "alpha" < "beta").
- Numeric < non-numeric (e.g., "1" < "alpha").
- Build metadata is completely ignored.

### API

- **Version** struct with exported fields: Major, Minor, Patch (int), Prerelease, Build (string).
- **ErrInvalid** sentinel error, wrapped by all Parse errors.
- **Compare** signature: `Compare(a, b Version) int`, compatible with `slices.SortFunc`.
- **String**, **IsPrerelease**, **Sort** methods as documented.

## Test Requirements

Write comprehensive tests in `semver_test.go`. The starter skeleton has room for your implementation.

### Table-Driven Tests

- **TestParse**: valid versions (plain, with v, prerelease, build, all combinations).
- **TestParseInvalid**: every kind of input the spec forbids. Go through the spec rule by rule.
- **TestMustParse**: panic on invalid, success on valid.
- **TestString**: round-trip and canonical form.
- **TestIsPrerelease**: correctly identify pre-releases.
- **TestCompare**: numeric order, prerelease < release, prerelease comparison rules, build ignored.
- **TestCompareSpecOrder**: the spec's example precedence list at §11.
- **TestSort**: stable sort of mixed versions.

### Fuzzing

- **FuzzParse**: seeds with valid/invalid versions; fuzz never panics, errors wrap ErrInvalid, valid versions round-trip (Parse → String → Parse).
- **FuzzCompare**: seeds with pairs; verify antisymmetry (Compare(a,b) == -Compare(b,a)).

### Examples

- **ExampleCompare**: show Compare and the -1/0/+1 result.
- **ExampleSort**: show Sort on a mixed slice.

### Benchmarks

- **BenchmarkParse**: typical version string.
- **BenchmarkCompare**: two pre-release versions.

## Coverage

Aim for ≥90% code coverage. Check with:

```bash
go test ./modules/04-testing/project/semver/ -cover
go test ./modules/04-testing/project/semver/ -coverprofile=c.out
go tool cover -html=c.out  # browser view
```

## Verification Checklist

- [ ] `go test ./modules/04-testing/project/semver/` passes all tests.
- [ ] `go vet ./modules/04-testing/project/semver/` passes.
- [ ] `go test -cover` shows ≥90% coverage.
- [ ] Fuzzing: `go test -run='^$' -fuzz=FuzzParse -fuzztime=30s` and `FuzzCompare` pass.
- [ ] Benchmarks run without panicking: `go test -bench=. -benchmem -run='^$'`.
- [ ] Code is `gofmt` clean.

## Stretch Goals

1. **Performance**: Optimize Parse or Compare and measure with benchmarks. Document trade-offs (e.g., pre-computing splits vs. parsing on demand).
2. **Zero-allocation Compare**: `Compare` allocates (it splits the pre-release strings). Write a `CompareFast` that walks both strings with `strings.IndexByte`, prove it equal to `Compare` with a differential fuzz test (`Compare` is the oracle), and show the benchmark difference.
3. **CLI Tool**: write a `cmd/semver` command-line tool that parses versions and sorts a list.

## Run Tests

```bash
# Compile and basic tests:
go test ./modules/04-testing/project/semver/

# Verbose output:
go test ./modules/04-testing/project/semver/ -v

# With race detector:
go test -race ./modules/04-testing/project/semver/

# Fuzzing (30 seconds):
go test -run='^$' -fuzz=FuzzParse -fuzztime=30s ./modules/04-testing/project/semver/

# Benchmarks:
go test -bench=. -benchmem -run='^$' ./modules/04-testing/project/semver/

# Coverage:
go test -cover ./modules/04-testing/project/semver/
go test -coverprofile=c.out ./modules/04-testing/project/semver/
go tool cover -html=c.out
```
