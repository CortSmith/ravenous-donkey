# Project: Optimize the Log Parser

## Overview

You're given a correct but slow HTTP access log parser. Your job is to optimize it to:
1. **Pass the allocation gate:** ≤60 allocations for 2000 log lines
2. **Non-scaling allocations:** allocations should not grow with the number of lines
3. **Reach ≥5x speedup** on a benchmark (measure with `-benchstat`)

The starter code is a working implementation with three regex-based validators and a naive parser.
It's correct but slow because:
- It validates with `regexp.MatchString` on every line
- It uses `strings.FieldsFunc` to split fields
- Every operation that looks simple allocates

Your optimization must keep the behavior identical.

## Specification

The `Summarize` function reads HTTP access logs from an `io.Reader` and aggregates statistics.

Each log line has **exactly six fields** separated by spaces or tabs:
```
<timestamp> <method> <path> <status> <duration_ms> <bytes>
2026-10-11T12:00:00Z GET /api/users 200 12 5120
```

A **well-formed** line has:
- Exactly 6 fields (non-empty, separated by ` ` or `\t`)
- Method: one or more uppercase ASCII letters (A-Z)
- Path: must start with `/`
- Status: exactly three digits in 100–599
- Duration and bytes: unsigned decimal integers fitting in `int64`

**Blank lines** (after trimming spaces, tabs, and carriage returns) are skipped and not counted.

**Malformed lines** are counted but otherwise ignored. They don't affect the summary statistics.

The function returns:
- `Lines`: count of all non-blank lines (well-formed + malformed)
- `Malformed`: count of rejected lines
- `Methods`: map of HTTP method → count
- `Status`: map of HTTP status code → count
- `Paths`: map of path → aggregated statistics (Count, TotalMillis, MaxMillis, Bytes)

## Acceptance Criteria

✅ **Correctness:**
- All tests pass: `go test ./modules/09-performance/project/`
- No data race: `go test -race ./modules/09-performance/project/`

✅ **Performance gate:**
- `TestSummarizeAllocs` passes: ≤60 allocations for 2000 lines
- `TestSummarizeAllocsDoNotScale` passes: allocations don't scale with line count

✅ **Benchmark improvement:**
- Create a `project/RESULTS.md` file documenting before/after benchmarks
- Show at least 5x speedup (or close to it)

## How to Approach This

### Step 1: Measure the Slow Version

Run the starter code's benchmark to see the baseline:

```bash
cd /home/csmith/Workspace/tools/go-course
go test -run=^$ -bench=Summarize -benchmem ./modules/09-performance/project/
```

Output (approximate):
```
BenchmarkSummarize    500    2500000 ns/op   1024 B/op   150 allocs/op
```

Note: your baseline will differ, but it's likely in the thousands of allocations.

### Step 2: Profile for Hotspots

Use pprof to see where time and allocations come from:

```bash
go test -run=^$ -bench=Summarize -memprofile=mem.out ./modules/09-performance/project/
go tool pprof -sample_index=alloc_objects -top mem.out
```

Or use the interactive UI:
```bash
go tool pprof -http=localhost:6060 mem.out
```

Look at the "top" allocators. You'll likely see:
- `regexp` package functions (each MatchString call allocates)
- `strings.FieldsFunc` (allocates a slice of fields)
- Maybe `fmt` or `strings` functions if the code isn't optimal yet

### Step 3: Optimize

**Common optimizations for this problem:**

#### 3a. Skip Regex Validation

The three regex validators can be replaced with simple character checks:

```go
// OLD: regexes on every line
//
//	if !methodRE.MatchString(f[1]) { ... }
//	if !statusRE.MatchString(f[3]) { ... }
//	if !numberRE.MatchString(f[4]) { ... }

// NEW: manual validation
func isMethod(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func isStatus(s string) bool {
	return len(s) == 3 && s[0] >= '1' && s[0] <= '5' &&
		s[1] >= '0' && s[1] <= '9' && s[2] >= '0' && s[2] <= '9'
}

func isDecimal(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

fmt.Println(isMethod("GET"), isMethod("get"), isStatus("404"), isStatus("600"), isDecimal("120"), isDecimal("-1"))
// true false true false true false
```

(The existing regex compilation is static, but calling MatchString still allocates. Skipping regex entirely is better.)

#### 3b. Replace FieldsFunc with Manual Parsing

`strings.FieldsFunc` allocates a slice of fields. Instead, find field boundaries manually:

```go
line := "2026-10-11T12:00:00Z GET /api/users 200 12 5120"

// OLD: allocates a []string for every line
f := strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == '\t' })
fmt.Println(f[1], f[2])

// NEW: record field boundaries in a fixed-size array (no allocation)
type field struct{ start, end int }
var fields [6]field
n, inField := 0, false
for i := 0; i <= len(line) && n <= 6; i++ {
	sep := i == len(line) || line[i] == ' ' || line[i] == '\t'
	switch {
	case !sep && !inField: // a field starts
		if n < 6 {
			fields[n].start = i
		}
		inField = true
	case sep && inField: // a field ends
		if n < 6 {
			fields[n].end = i
		}
		n++
		inField = false
	}
}
method := line[fields[1].start:fields[1].end] // substrings share line's memory: no copy
path := line[fields[2].start:fields[2].end]
fmt.Println(method, path, n == 6)
```

Or use a simpler approach: count separators and extract fields by index:

```go
// entry is one parsed log line (as in logstats.go).
type entry struct {
	method   string
	path     string
	status   int
	duration int64
	bytes    int64
}

func parseLineManual(line string) (entry, error) {
	var parts [6]string
	partIdx := 0
	start := 0

	for i := 0; i <= len(line); i++ {
		if i == len(line) || line[i] == ' ' || line[i] == '\t' {
			if i > start { // non-empty field
				if partIdx == 6 {
					return entry{}, errors.New("too many fields")
				}
				parts[partIdx] = line[start:i]
				partIdx++
			}
			start = i + 1
		}
	}
	if partIdx != 6 {
		return entry{}, errors.New("too few fields")
	}
	// validate and extract parts[0..5] (validation left to you)
	status, _ := strconv.Atoi(parts[3])
	dur, _ := strconv.ParseInt(parts[4], 10, 64)
	size, _ := strconv.ParseInt(parts[5], 10, 64)
	return entry{method: parts[1], path: parts[2], status: status, duration: dur, bytes: size}, nil
}

func main() {
	fmt.Println(parseLineManual("2026-10-11T12:00:00Z GET /api/users 200 12 5120"))
	fmt.Println(parseLineManual("2026-10-11T12:00:00Z GET /api/users 200"))
}
```

#### 3c. Use strconv Directly

When parsing integers, `strconv.Atoi` is simpler than `strconv.ParseInt`. But don't parse unless you've validated the format:

```go
parts := [6]string{"2026-10-11T12:00:00Z", "GET", "/", "404", "3", "0"}
// Only call Atoi if isStatus/isDecimal returned true:
status, _ := strconv.Atoi(parts[3]) // safe, no error possible if validated
fmt.Println(status)                 // 404
```

Or use `strconv.ParseInt` with error handling as a fallback.

#### 3d. Preallocate Maps

The maps `Methods`, `Status`, and `Paths` start empty and grow. If you can estimate their final size, preallocate:

```go
// The types, from logstats.go:
import logstats "gocourse/modules/09-performance/project"

type (
	Summary   = logstats.Summary
	PathStats = logstats.PathStats
)

s := Summary{
	Methods: make(map[string]int, 10),        // guess: ~10 distinct methods
	Status:  make(map[int]int, 10),           // guess: ~10 distinct status codes
	Paths:   make(map[string]PathStats, 100), // guess: ~100 distinct paths
}
fmt.Println(len(s.Methods), len(s.Paths)) // 0 0: sized, but still empty
```

In tests, this reduces growth-related allocations.

### Step 4: Verify

Run the tests and benchmark again:

```bash
go test ./modules/09-performance/project/ -v
go test -race ./modules/09-performance/project/
go test -run=^$ -bench=Summarize -benchmem ./modules/09-performance/project/
```

### Step 5: Document Results

Create `project/RESULTS.md` with before/after benchmarks:

```markdown
# Optimization Results

## Before (Slow)
- 2500 ns/op, 1024 B/op, 150 allocs/op
- Profile: most time in regexp, most allocations in FieldsFunc + regex

## After (Optimized)
- 400 ns/op, 64 B/op, 40 allocs/op
- Speedup: 6.25x
- Profile: most time in field parsing and map updates (acceptable)

## Changes
1. Removed regexes for method/status/number validation
2. Replaced strings.FieldsFunc with manual field parsing
3. Prealloc'd maps with hints
```

## Tips

- **Start simple:** remove the regexes first. That's the biggest win.
- **Profile before and after:** compare the "top" output to see what you've fixed.
- **Test stability:** run with `-count=10` to see variance. If it's huge, you might be on a noisy machine.
- **Readable code:** the solution's approach is still readable. Don't sacrifice too much clarity.
- **Edge cases:** the test suite is comprehensive. If you introduce a bug, the tests will catch it.

## Expected Performance

With all optimizations applied, you should see:
- **Allocations:** 30–60 per 2000 lines (depending on distinct paths/methods)
- **Speed:** 5–10x faster than the starter code
- **Allocations do not scale:** 1000 lines and 4000 lines should have roughly the same alloc count

## Stretch Goal

After optimizing, can you identify any remaining allocations using pprof? Some are unavoidable (e.g., storing new
map keys), but the goal is to show that the per-line cost approaches zero as the number of lines grows.

## Run the Test Command

The exact test command to verify everything works:

```bash
go test ./modules/09-performance/project/ -v -race
```

All tests must pass before you consider the project complete.
