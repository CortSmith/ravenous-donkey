# Review Guide: Module 09 Solutions

This guide explains the readable and optimized solutions for each exercise and the project,
along with a grading checklist.

---

## Exercise 1: appendints

### Readable Solution

```go
// appendints.go
func AppendInts(dst []byte, nums []int, sep byte) []byte {
    for i, n := range nums {
        if i > 0 {
            dst = append(dst, sep)
        }
        dst = strconv.AppendInt(dst, int64(n), 10)
    }
    return dst
}
```

**Why it's idiomatic:**
- `strconv.AppendInt` is the standard way to append a formatted integer to a `[]byte`
- No intermediate string allocation
- No interface boxing
- Direct write to the slice

### Optimized Solution

The readable solution *is* the optimized one. There's no slower-but-more-readable variant here,
because the obvious approach is already optimal.

The starter code used `fmt.Sprintf`, which is slow because:
1. It boxes the integer into `interface{}`
2. It allocates a new string
3. It converts the string to `[]byte` for appending (another allocation)

The solution avoids all three.

### Benchmark Results

**Starter (slow):**
```
BenchmarkAppendInts-20    80280    14775 ns/op    3202 B/op    400 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Solution (optimized):**
```
BenchmarkAppendInts-20   535849     2198 ns/op       0 B/op      0 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Speedup:** 6.72x, zero allocations when dst has capacity.

### Trade-off

**Use the optimized version always.** `strconv.AppendInt` is not less readable than `fmt.Sprintf`;
it's actually clearer about intent (formatting into a buffer, not a string).

### Review Checklist

- [ ] Student chose `strconv.AppendInt` over `fmt.Sprintf`
- [ ] No intermediate string allocation
- [ ] Handles negative numbers correctly (test includes negatives)
- [ ] Preserves dst prefix (test includes prefix case)
- [ ] Separators placed correctly (no leading/trailing separator)
- [ ] Works with empty nums (returns dst unchanged)
- [ ] AllocsPerRun test passes (0 allocations)

---

## Exercise 2: csvline

### Readable Solution

```go
// csvline.go
func Line(fields []string) string {
    var sb strings.Builder
    sb.Grow(estimateLen(fields))
    for i, f := range fields {
        if i > 0 {
            sb.WriteByte(',')
        }
        if strings.ContainsAny(f, ",\"\r\n") {
            sb.WriteByte('"')
            for _, c := range f {
                sb.WriteRune(c)
                if c == '"' {
                    sb.WriteRune('"')  // double quotes
                }
            }
            sb.WriteByte('"')
        } else {
            sb.WriteString(f)
        }
    }
    return sb.String()
}

func estimateLen(fields []string) int {
    total := len(fields) - 1  // separators
    for _, f := range fields {
        total += len(f)
        if strings.ContainsAny(f, ",\"\r\n") {
            total += 2  // quotes
            total += strings.Count(f, "\"")  // extra quotes
        }
    }
    return total
}
```

**Why it's idiomatic:**
- Uses `strings.Builder` to avoid repeated allocations
- Calls `Grow` to preallocate the exact size (no growth during writes)
- Clear control flow: quoted vs. unquoted fields

### Fast Variant (in fast.go)

```go
// fast.go
func AppendLine(dst []byte, fields []string) []byte {
    for i, f := range fields {
        if i > 0 {
            dst = append(dst, ',')
        }
        quote := false
        for j := 0; j < len(f); j++ {
            if c := f[j]; c == ',' || c == '"' || c == '\r' || c == '\n' {
                quote = true
                break
            }
        }
        if !quote {
            dst = append(dst, f...)
            continue
        }
        dst = append(dst, '"')
        start := 0
        for j := 0; j < len(f); j++ {
            if f[j] == '"' {
                dst = append(dst, f[start:j+1]...)
                dst = append(dst, '"')
                start = j + 1
            }
        }
        dst = append(dst, f[start:]...)
        dst = append(dst, '"')
    }
    return dst
}
```

**Why it's faster:**
- Returns `[]byte` instead of `string`, so the caller can reuse the buffer
- Scans each field byte-by-byte instead of calling `ContainsAny` and `Count`
- Avoids the string `→ rune` conversion in quoted fields
- For servers handling many records, the buffer can be reused, eliminating allocation

### Benchmark Results

**Starter:**
```
BenchmarkLine-20    153298     8255 ns/op   18320 B/op    118 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Readable (strings.Builder):**
```
BenchmarkLine-20    389277     2767 ns/op     448 B/op      1 alloc/op
```
(measured on go1.24.2 linux/amd64)

**Fast (AppendLine):**
```
BenchmarkLineFast/AppendLine-20   2480949    483.5 ns/op     0 B/op      0 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Speedup:** ~2.98x from starter to readable; ~5.72x from readable to fast; ~17.07x from starter to fast.

### Trade-off

- **Readable:** Use when `Line` is called rarely or in isolation. One allocation per call is acceptable.
- **Fast:** Use when encoding many records in a loop (e.g., server endpoint). Reuse the buffer:
  ```go
  buf := make([]byte, 0, 4096)
  for _, rec := range records {
      buf = AppendLine(buf[:0], rec.Fields)
      w.Write(buf)
  }
  ```

### Review Checklist

- [ ] Student chose `strings.Builder` over `+=` loop
- [ ] Calls `Builder.Grow` with estimated size (no growth during writes)
- [ ] Handles quoted fields correctly (commas, quotes, newlines)
- [ ] Doubles internal quotes (`"` → `""`)
- [ ] Produces valid CSV (RFC 4180 compliant)
- [ ] Empty fields handled correctly
- [ ] AllocsPerRun test passes (≤1 allocation)

---

## Exercise 3: dedupe

### Readable Solution

```go
// dedupe.go
func Unique(items []string) []string {
    seen := make(map[string]bool, len(items))
    out := make([]string, 0, len(items))
    for _, it := range items {
        if !seen[it] {
            seen[it] = true
            out = append(out, it)
        }
    }
    return out
}
```

**Why it's idiomatic:**
- Prealloc'd map and slice with capacity hints (no growth)
- Simple logic: track seen items, append new ones
- Returns a slice (consumer-friendly)

### Optimized Solution

The readable solution *is* already well-optimized. The starter code's main inefficiency is the lack of preallocation.

**Comparison:**

**Starter (no prealloc):**
```go
seen := map[string]bool{}          // empty, will grow
var out []string                   // nil, will grow
for _, it := range items { ... }
```

**Optimized (prealloc):**
```go
seen := make(map[string]bool, len(items))   // size hint
out := make([]string, 0, len(items))        // capacity
for _, it := range items { ... }
```

With preallocation, the map is sized upfront, and the slice grows at most once or twice.
Without it, both grow repeatedly, causing multiple allocations and rehashing.

### Benchmark Results

**Starter (2000 items, 1000 distinct):**
```
BenchmarkUnique-20    10000    102688 ns/op    143946 B/op    31 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Optimized:**
```
BenchmarkUnique-20    24372     55409 ns/op    141985 B/op    10 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Speedup:** 1.85x, allocations drop from 31 to 10.

### Trade-off

Always use the optimized version (with preallocation). The readability is unchanged; you're just adding capacity hints.

### Review Checklist

- [ ] Map prealloc'd with `make(map[...], len(items))`
- [ ] Slice prealloc'd with `make([]..., 0, len(items))`
- [ ] Preserves order of first occurrence
- [ ] Handles nil/empty input correctly (returns nil or empty slice)
- [ ] No duplicates in output
- [ ] AllocsPerRun test passes (≤12 allocs for typical input)

---

## Exercise 4: lineenc

### Readable Solution

```go
// lineenc.go
type Encoder struct {
    w   io.Writer
    buf []byte
}

func NewEncoder(w io.Writer) *Encoder {
    return &Encoder{w: w, buf: make([]byte, 0, 4096)}
}

func (e *Encoder) Encode(r Record) error {
    e.buf = e.buf[:0]  // reset, keep capacity
    
    e.buf = append(e.buf, "name="...)
    e.buf = strconv.AppendQuote(e.buf, r.Name)
    e.buf = append(e.buf, " score="...)
    e.buf = strconv.AppendInt(e.buf, int64(r.Score), 10)
    e.buf = append(e.buf, " passed="...)
    e.buf = strconv.AppendBool(e.buf, r.Passed)
    e.buf = append(e.buf, '\n')
    
    _, err := e.w.Write(e.buf)
    return err
}
```

**Why it's idiomatic:**
- Reuses a buffer in the Encoder (steady-state allocation-free)
- Uses `strconv.Append*` functions (no intermediate strings)
- Resets with `buf[:0]` to keep capacity
- Clear and straightforward

### Optimized Variant (in fast.go)

The readable solution is already near-optimal. A minor speedup could be:
- Using `unsafe` to avoid a final `string()` conversion (not recommended for this module)
- Preallocating larger if records are typically large
- Using `io.WriteString` for string literals (avoids `append(..., []byte...)`)

**Minor optimization:**
```go
func (e *Encoder) Encode(r Record) error {
    e.buf = e.buf[:0]
    e.buf = append(e.buf, "name="...)
    e.buf = strconv.AppendQuote(e.buf, r.Name)
    // ... etc., but use append with byte slices directly, no string conversions
    _, err := e.w.Write(e.buf)
    return err
}
```

The fast.go file in solutions shows this pattern, which shaves off a few percent.

### Benchmark Results

**Starter:**
```
BenchmarkEncode-20    3659322    305.3 ns/op    152 B/op    4 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Readable (with buffer reuse):**
```
BenchmarkEncode-20   15327054     78.06 ns/op      0 B/op    0 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Speedup:** 3.91x, zero allocations in steady state.

### Trade-off

Always use the buffer-reuse approach. It's not less readable, and it's essential for servers that encode many records.

### Review Checklist

- [ ] Added `buf []byte` field to Encoder
- [ ] Buffer prealloc'd to reasonable size (e.g., 4 KB)
- [ ] Resets with `buf[:0]` to keep capacity
- [ ] Uses `strconv.AppendQuote` for the name
- [ ] Uses `strconv.AppendInt` for the score
- [ ] Uses `strconv.AppendBool` for the passed flag
- [ ] Appends `\n` at the end
- [ ] AllocsPerRun test passes (0 allocations in steady state)
- [ ] Output format is exact (space separators, no extra spaces)

---

## Exercise 5: semver

### Readable Solution (fast.go)

Instead of a regex, manual parsing is simpler and faster:

```go
// fast.go
func Parse(s string) (Version, error) {
    // Strip optional "v" prefix
    if len(s) > 0 && s[0] == 'v' {
        s = s[1:]
    }
    
    // Find the first dot
    dot1 := strings.Index(s, ".")
    if dot1 == -1 {
        return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
    }
    
    major, err := strconv.Atoi(s[:dot1])
    if err != nil || major < 0 {
        return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
    }
    
    // Find the second dot
    dot2 := strings.Index(s[dot1+1:], ".")
    if dot2 == -1 {
        return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
    }
    dot2 += dot1 + 1
    
    minor, err := strconv.Atoi(s[dot1+1 : dot2])
    if err != nil || minor < 0 {
        return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
    }
    
    patch, err := strconv.Atoi(s[dot2+1:])
    if err != nil || patch < 0 {
        return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
    }
    
    return Version{Major: major, Minor: minor, Patch: patch}, nil
}
```

**Why it's better:**
- No regex compilation
- No `FindStringSubmatch` allocation
- `strings.Index` and `strconv.Atoi` are fast and well-optimized
- Simpler error messages

### Optimized Variant (even more compact)

Using `strings.Cut` (Go 1.18+) makes it cleaner:

```go
// Slightly cleaner version
func Parse(s string) (Version, error) {
    if len(s) > 0 && s[0] == 'v' {
        s = s[1:]
    }
    
    before, after, ok := strings.Cut(s, ".")
    if !ok {
        return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
    }
    major, err := strconv.Atoi(before)
    if err != nil || major < 0 {
        return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
    }
    
    before, patch, ok := strings.Cut(after, ".")
    if !ok {
        return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
    }
    minor, err := strconv.Atoi(before)
    if err != nil || minor < 0 {
        return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
    }
    
    patchInt, err := strconv.Atoi(patch)
    if err != nil || patchInt < 0 {
        return Version{}, fmt.Errorf("semver: parse %q: %w", s, ErrInvalid)
    }
    
    return Version{Major: major, Minor: minor, Patch: patchInt}, nil
}
```

### Benchmark Results

**Starter (regex compiled per call):**
```
BenchmarkParse-20    152104     11656 ns/op   11257 B/op    136 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Readable (manual parsing with strings.Cut):**
```
BenchmarkParse-20   31144882      39.46 ns/op      0 B/op      0 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Speedup:** 295.2x, zero allocations.

### Trade-off

The manual parsing version is actually *more* readable than the regex, because the format is simple.
Always prefer it.

### Review Checklist

- [ ] Student removed the regex or hoisted it to a package variable
- [ ] Uses `strings.Index` or `strings.Cut` to find dots
- [ ] Validates component format (no leading zeros, valid range)
- [ ] Handles optional "v" prefix
- [ ] Uses `strconv.Atoi` for parsing (or error handling if Atoi fails)
- [ ] Rejects invalid formats (wrong number of dots, non-decimal characters)
- [ ] AllocsPerRun test passes (0 allocations for valid input)
- [ ] Error messages are clear

---

## Project: Optimize the Log Parser

### Readable Solution (in logstats.go)

The starter code validates with regexes and parses with `strings.FieldsFunc`.
The readable solution keeps the same structure but:

1. **Compile regexes once** at package level
2. **Keep the FieldsFunc approach** (it's fast enough once regexes aren't the bottleneck)

```go
// logstats.go (partial)
var (
    methodRE = regexp.MustCompile(`^[A-Z]+$`)
    statusRE = regexp.MustCompile(`^[1-5][0-9][0-9]$`)
    numberRE = regexp.MustCompile(`^[0-9]+$`)
)

// parseLine (same as before, but regexes are compiled once)
func parseLine(line string) (entry, error) {
    f := strings.FieldsFunc(line, isSep)
    if len(f) != 6 {
        return entry{}, fmt.Errorf("want 6 fields, got %d", len(f))
    }
    if !methodRE.MatchString(f[1]) {  // compiled once, reused
        return entry{}, fmt.Errorf("bad method %q", f[1])
    }
    // ... etc.
}
```

This alone cuts allocations dramatically (regex compilation is expensive).

### Optimized Solution (fast.go)

Replace regexes and FieldsFunc with manual parsing:

```go
// fast.go
func parseLineOptimized(line string) (entry, error) {
    // Manual field extraction: scan for separators
    var fieldStarts [6]int
    var fieldEnds [6]int
    fieldIdx := 0
    inField := false
    
    for i := 0; i < len(line) && fieldIdx < 6; i++ {
        c := line[i]
        if c == ' ' || c == '\t' {
            if inField {
                fieldEnds[fieldIdx] = i
                fieldIdx++
                inField = false
            }
        } else {
            if !inField {
                fieldStarts[fieldIdx] = i
                inField = true
            }
        }
    }
    if inField {
        fieldEnds[fieldIdx] = len(line)
        fieldIdx++
    }
    
    if fieldIdx != 6 {
        return entry{}, fmt.Errorf("want 6 fields, got %d", fieldIdx)
    }
    
    // Extract fields as substrings (no allocation)
    method := line[fieldStarts[1]:fieldEnds[1]]
    path := line[fieldStarts[2]:fieldEnds[2]]
    statusStr := line[fieldStarts[3]:fieldEnds[3]]
    durStr := line[fieldStarts[4]:fieldEnds[4]]
    bytesStr := line[fieldStarts[5]:fieldEnds[5]]
    
    // Validate method: all uppercase letters
    if !isMethod(method) {
        return entry{}, fmt.Errorf("bad method %q", method)
    }
    
    // Validate path: starts with /
    if len(path) == 0 || path[0] != '/' {
        return entry{}, fmt.Errorf("bad path %q", path)
    }
    
    // Validate status: exactly 3 digits, 100-599
    status := parseStatus(statusStr)
    if status < 100 || status > 599 {
        return entry{}, fmt.Errorf("bad status %q", statusStr)
    }
    
    // Validate and parse duration
    dur, err := strconv.ParseInt(durStr, 10, 64)
    if err != nil {
        return entry{}, fmt.Errorf("bad duration: %w", err)
    }
    
    // Validate and parse bytes
    size, err := strconv.ParseInt(bytesStr, 10, 64)
    if err != nil {
        return entry{}, fmt.Errorf("bad bytes: %w", err)
    }
    
    return entry{method: method, path: path, status: status, duration: dur, bytes: size}, nil
}

func isMethod(s string) bool {
    if len(s) == 0 {
        return false
    }
    for i := 0; i < len(s); i++ {
        c := s[i]
        if c < 'A' || c > 'Z' {
            return false
        }
    }
    return true
}

func parseStatus(s string) int {
    if len(s) != 3 {
        return -1
    }
    if s[0] < '1' || s[0] > '5' || s[1] < '0' || s[1] > '9' || s[2] < '0' || s[2] > '9' {
        return -1
    }
    return int(s[0]-'0')*100 + int(s[1]-'0')*10 + int(s[2]-'0')
}
```

**Why it's faster:**
- No regex compilation or MatchString calls
- No `FieldsFunc` allocation
- Field extraction by substring (zero-copy)
- Validation is O(field length), not regex matching
- Preallocated maps reduce insertion-time allocations

### Benchmark Results

**Starter:**
```
BenchmarkSummarize-20    176    6624259 ns/op    1560332 B/op    20903 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Optimized (manual parsing):**
```
BenchmarkSummarize-20   1048    1129365 ns/op       5437 B/op       30 allocs/op
```
(measured on go1.24.2 linux/amd64)

**Speedup:** 5.86x from starter to optimized.

### Trade-off

- **Readable:** Hoist regexes to package level. Simpler to understand, 3x speedup.
- **Optimized:** Manual parsing. Slightly longer code, but 6x speedup and zero per-line allocations.

For this module, the optimized version is worth it because parsing is the core operation.

### Review Checklist

- [ ] **Correctness:**
  - All tests pass: `go test ./modules/09-performance/project/ -v`
  - No race conditions: `go test -race ./modules/09-performance/project/`
  - Handles empty input (empty lines, blank input)
  - Counts malformed lines correctly
  - Never allocates nil maps (they're initialized)

- [ ] **Performance:**
  - `TestSummarizeAllocs` passes (≤60 allocations for 2000 lines)
  - `TestSummarizeAllocsDoNotScale` passes (allocations don't scale with line count)
  - Benchmark shows ≥5x improvement over starter
  - `project/RESULTS.md` documents before/after numbers

- [ ] **Optimization technique:**
  - Regexes are either hoisted or removed
  - `FieldsFunc` is replaced or optimized (no per-field allocations)
  - Maps are prealloc'd
  - Manual parsing or `strings.Cut` used instead of `FieldsFunc`

- [ ] **Code quality:**
  - No panics on malformed input
  - Error messages are clear (helpful for debugging)
  - Handles edge cases (leading zeros, large numbers, unicode paths)
  - Code is reasonably readable (even if optimized)

---

## Summary: When to Optimize

**Readable (first solution):** Choose this when the code is clear and performance is acceptable.

**Optimized (fast variant):** Choose this when:
- The code is called frequently (1000+ times per second)
- Allocations are visible in a profile
- Readability loss is acceptable (e.g., for core parsers or encoders)

For this module, all exercises and the project benefit from optimization because they're either in hot loops or process many items. The optimized versions are justified.

**Grading note:** A student who implements the readable solution and passes the allocation test has met the goal.
A student who goes further and explores fast.go variants has gone above and beyond.
