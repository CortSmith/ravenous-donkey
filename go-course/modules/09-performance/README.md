# Module 09: Measure, Profile, Optimize

## Goals

After this module, you'll be able to:
- Write benchmarks with Go's standard `testing` package and measure allocation counts
- Use `pprof` (CPU and memory profiling) to identify performance bottlenecks
- Understand allocation sources: interface boxing, string concatenation, regex compilation, and sub-optimal data structures
- Optimize code with techniques like buffer reuse, preallocation, and algorithm choice
- Balance readability against performance using data-driven trade-off analysis
- Recognize when optimization is premature and when it's essential

## Concepts

### The Golden Rule: Measure First

The mantra in Go's performance culture is **"measure first."** It's easy to guess wrong about where the time or memory goes.
Optimize blindly, and you waste hours on the slow path instead of the critical one. Profile, identify the hotspot, then optimize.

```text
// Guess: "for loops are slow"
// Fact: your parser is calling regexp.MustCompile 1000 times per line
```

Go gives you excellent tools for this: `testing` benchmarks with built-in allocation reporting,
and `pprof` for CPU and memory profiling of real workloads.

### Benchmarks: Measuring Throughput and Allocations

Go 1.24 introduced `testing.B.Loop`, which makes benchmarks simpler and more idiomatic:

```go
// CORRECT (Go 1.24+)
func BenchmarkJoin(b *testing.B) {
	parts := makeParts(1000)
	b.ReportAllocs()
	for b.Loop() {
		_ = strings.Join(parts, ",")
	}
}

// OLD STYLE (still works, but less idiomatic)
func BenchmarkJoinOld(b *testing.B) {
	parts := makeParts(1000)
	b.ReportAllocs()
	b.ResetTimer() // b.N style needs this to exclude the setup; b.Loop does it for you
	for i := 0; i < b.N; i++ {
		_ = strings.Join(parts, ",")
	}
}

func makeParts(n int) []string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "part" + strconv.Itoa(i)
	}
	return parts
}
```

When you run `go test -bench=. -benchmem`, you get output like:
```
BenchmarkJoin-8    12345    123456 ns/op    45678 B/op    3 allocs/op
```

- `123456 ns/op`: nanoseconds per operation
- `45678 B/op`: bytes allocated per operation
- `3 allocs/op`: number of heap allocations per operation

**Key insight:** Even if an optimization saves only 1 ns per operation, if you shave off one allocation, you save GC
pauses and peak memory in the long run. Allocations matter.

### testing.AllocsPerRun: Gates for Correctness *and* Performance

In this course, each performance exercise uses `testing.AllocsPerRun` to set a performance *gate*: a test
that fails if the code is too slow.

```go
func TestAppendIntsAllocs(t *testing.T) {
	nums := testNums(200)
	dst := make([]byte, 0, 8192)
	want := string(AppendInts(nil, nums, ','))

	allocs := testing.AllocsPerRun(100, func() {
		dst = AppendInts(dst[:0], nums, ',')
	})

	if string(dst) != want {
		t.Fatalf("output changed between calls")
	}
	if allocs != 0 {
		t.Errorf("AppendInts allocates %.0f times, want 0", allocs)
	}
}

// AppendInts appends nums to dst, separated by sep.
func AppendInts(dst []byte, nums []int, sep byte) []byte {
	for i, n := range nums {
		if i > 0 {
			dst = append(dst, sep)
		}
		dst = strconv.AppendInt(dst, int64(n), 10)
	}
	return dst
}

func testNums(n int) []int {
	nums := make([]int, n)
	for i := range nums {
		nums[i] = i * 7919
	}
	return nums
}
```

This test:
1. Runs the function 100 times (to amortize noise)
2. Counts the **average** number of allocations per run
3. Fails the student until they achieve the threshold

The threshold is set so the starter code clearly fails, and the optimal solution clearly passes.

**Run it:**
```bash
go test ./modules/09-performance/exercises/appendints/ -v
```

The test initially fails and shows a hint about the optimization strategy.

### Escape Analysis: When Values Escape to the Heap

Some allocations are hard to spot by reading code. Go's escape analyzer decides whether a variable can live on the stack
or must be moved ("escape") to the heap. Understanding what escapes helps you identify unnecessary allocations.

Compile with `-gcflags=-m` to see escape analysis decisions:

```bash
go test -run=^$ -bench=. -gcflags=-m ./modules/09-performance/exercises/appendints/ 2>&1 | head -30
```

Real output on go1.24.2 for the **starter** (`go build -gcflags=-m ./modules/09-performance/exercises/appendints/`):
```
appendints.go:17:17: leaking param: dst to result ~r0 level=0
appendints.go:17:29: nums does not escape
appendints.go:22:32: ... argument does not escape
appendints.go:22:39: n escapes to heap
```

And for the **solution** (`strconv.AppendInt`):
```
appendints.go:13:17: leaking param: dst to result ~r0 level=0
appendints.go:13:29: nums does not escape
```

**Interpretation:**
- `n escapes to heap`: passing an `int` to `fmt.Sprintf` boxes it into an `interface{}` (`any`), and that box is heap-allocated on **every call**. This is the hidden allocation in the starter.
- `... argument does not escape`: the variadic `[]any` slice itself stays on the stack; only the boxed value escapes.
- `leaking param: dst to result`: `dst` flows into the return value. That is expected for any `append`-style API and costs nothing by itself.
- `does not escape`: stays on the stack, so it's free for the GC.

In the solution, the `n escapes to heap` line is simply gone. That's how you confirm an optimization removed an allocation before you even run a benchmark.

### Allocation Sources: The Usual Suspects

#### 1. Interface Boxing

When a value is stored in an `interface{}` or a concrete interface, the runtime must "box" it:
allocate a small header holding the type and a pointer to the value.

```go
func TestFormatAllocs(t *testing.T) {
	n := 123456
	dst := make([]byte, 0, 32)

	// BAD: fmt.Sprintf boxes n into interface{} and allocates the result string
	bad := testing.AllocsPerRun(100, func() { _ = fmt.Sprintf("%d", n) })

	// GOOD: strconv.AppendInt never allocates (when dst has capacity)
	good := testing.AllocsPerRun(100, func() { dst = strconv.AppendInt(dst[:0], int64(n), 10) })

	t.Logf("Sprintf: %.0f allocs, AppendInt: %.0f allocs", bad, good) // 2 vs 0: the boxed int and the result string
}
```

**Lesson:** Use `strconv` for number formatting, not `fmt`. `strconv.Append*` functions write directly
into a `[]byte`, avoiding the intermediate string and the boxing.

#### 2. String Concatenation in Loops

Each `s += x` allocates a new string, copies the old one, and discards the old one. O(n²) behavior:

```go
func joinBad(words []string) string {
	// BAD: quadratic
	s := ""
	for _, w := range words {
		s += w + ","
	}
	return s
}

func joinGood(words []string, estimatedLen int) string {
	// GOOD: linear
	var sb strings.Builder
	sb.Grow(estimatedLen)
	for i, w := range words {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(w)
	}
	return sb.String()
}

func BenchmarkJoin(b *testing.B) {
	words := strings.Fields(strings.Repeat("lorem ipsum dolor ", 300))
	b.Run("concat", func(b *testing.B) {
		for b.Loop() {
			joinBad(words)
		}
	})
	b.Run("builder", func(b *testing.B) {
		for b.Loop() {
			joinGood(words, 6*len(words))
		}
	})
}
```

`strings.Builder.Grow(n)` preallocates the exact size, then writes to the internal buffer without further allocation.

#### 3. Regular Expression Compilation

`regexp.MustCompile` and `regexp.Compile` are expensive: they parse the pattern and build a finite automaton.
Doing this once per call is wasteful.

```go
type Version struct{ Major, Minor, Patch int }

func parseWith(re *regexp.Regexp, s string) (Version, error) {
	if !re.MatchString(s) {
		return Version{}, fmt.Errorf("bad version %q", s)
	}
	var v Version
	fmt.Sscanf(s, "%d.%d.%d", &v.Major, &v.Minor, &v.Patch)
	return v, nil
}

// BAD: recompiles every call
func ParseSlow(s string) (Version, error) {
	re := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	return parseWith(re, s)
}

// GOOD: compile once at package init
var versionRE = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func Parse(s string) (Version, error) {
	return parseWith(versionRE, s)
}

// EVEN BETTER: avoid regex altogether
func ParseManual(s string) (Version, error) {
	var nums [3]int
	for i := range nums {
		part, rest, found := strings.Cut(s, ".")
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || (i < 2) != found {
			return Version{}, fmt.Errorf("bad version %q", s)
		}
		nums[i], s = n, rest
	}
	return Version{nums[0], nums[1], nums[2]}, nil
}

func BenchmarkParse(b *testing.B) {
	for name, parse := range map[string]func(string) (Version, error){
		"slow": ParseSlow, "precompiled": Parse, "manual": ParseManual,
	} {
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				parse("1.24.2")
			}
		})
	}
}
```

#### 4. Unprealloc'd Maps and Slices

Maps and slices grow by doubling their backing arrays. Each growth allocates and rehashes/copies.

```go
func BenchmarkDedupe(b *testing.B) {
	items := make([]string, 500) // mostly distinct, as in a typical dedupe
	for i := range items {
		items[i] = strconv.Itoa(i % 450)
	}

	// BAD: grow step by step
	b.Run("grow", func(b *testing.B) {
		for b.Loop() {
			seen := map[string]bool{} // starts empty
			var out []string
			for _, x := range items {
				if !seen[x] {
					seen[x] = true
					out = append(out, x)
				}
			}
		}
	})

	// GOOD: preallocate
	b.Run("prealloc", func(b *testing.B) {
		for b.Loop() {
			seen := make(map[string]bool, len(items))
			out := make([]string, 0, len(items))
			for _, x := range items {
				if !seen[x] {
					seen[x] = true
					out = append(out, x)
				}
			}
		}
	})
}
```

With preallocation, the map is sized to fit all entries and the slice never grows. On this input, allocations drop
from about 25 to 4 per call and the time to about a third. When the input is full of duplicates, though, `len(items)`
overestimates the result and preallocating can cost more than it saves. Measure with your real data.

#### 5. Reusable Buffers

When encoding or building structured output repeatedly (e.g., in a server), keep a `[]byte` buffer in the struct
and reset it for each operation:

```go
type Record struct {
	Name string
	Age  int
}

type Encoder struct {
	w   io.Writer
	buf []byte // reusable buffer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w, buf: make([]byte, 0, 4096)}
}

func (e *Encoder) Encode(r Record) error {
	e.buf = e.buf[:0] // reset, keep capacity
	e.buf = append(e.buf, "name="...)
	e.buf = append(e.buf, r.Name...)
	e.buf = append(e.buf, " age="...)
	e.buf = strconv.AppendInt(e.buf, int64(r.Age), 10)
	e.buf = append(e.buf, '\n')
	_, err := e.w.Write(e.buf)
	return err
}

func main() {
	enc := NewEncoder(os.Stdout)
	enc.Encode(Record{"Ada", 36})
	enc.Encode(Record{"Grace", 85})
	allocs := testing.AllocsPerRun(100, func() { NewEncoder(io.Discard).Encode(Record{"x", 1}) })
	fmt.Println("allocs per NewEncoder+Encode:", allocs) // the buffer; Encode itself adds none
}
```

After the first call, `Encode` never allocates (assuming records fit in 4 KB).

### CPU Profiling with pprof

Benchmarking tells you throughput and allocation counts, but not *where* the time goes.
CPU profiling samples the call stack and shows you the hot path.

```bash
go test -run=^$ -bench=Summarize -cpuprofile=cpu.out ./modules/09-performance/project/
go tool pprof -http=localhost:6060 cpu.out
```

This opens an interactive web UI showing:
- **Flame graph:** the call tree colored by time spent
- **Top table:** functions consuming the most CPU time
- **List view:** source code annotated with percentage of time on each line

Or use the text interface:
```bash
go tool pprof cpu.out
(pprof) top10
(pprof) list parseLine
(pprof) exit
```

The `-list` command shows the offending function with time percentages next to each line.

### Memory Profiling

To see where allocations come from:

```bash
go test -run=^$ -bench=Summarize -memprofile=mem.out ./modules/09-performance/project/
go tool pprof -sample_index=alloc_objects -top mem.out
```

This reports the count and size of allocations by call site. The `-sample_index=alloc_objects` flag
counts the number of allocations (default is bytes allocated).

```
Type: alloc_objects
Showing nodes accounting for 4364070, 99.95% of 4366278 total
      flat  flat%   sum%        cum   cum%
   2173964 49.79% 49.79%    2173964 49.79%  strings.FieldsFunc
   2037189 46.66% 96.45%    2037189 46.66%  bufio.(*Scanner).Text
     54613  1.25% 97.70%      87576  2.01%  fmt.Errorf
     32768  0.75% 98.45%      32768  0.75%  errors.New (inline)
```

The same run's CPU profile (`go tool pprof -top logstats.test cpu.out`) for comparison:
```
Showing nodes accounting for 1000ms, 64.52% of 1550ms total
      flat  flat%   sum%        cum   cum%
     530ms 34.19% 34.19%      820ms 52.90%  regexp.(*Regexp).doOnePass
     150ms  9.68% 43.87%      300ms 19.35%  strings.FieldsFunc
      70ms  4.52% 48.39%       70ms  4.52%  regexp/syntax.(*Inst).MatchRunePos
      70ms  4.52% 52.90%       70ms  4.52%  strconv.ParseUint
```
(Real output from the starter project on go1.24.2 linux/amd64; your numbers will differ.)

Read them together. **Time** is dominated by the regexp engine (53% cumulative), so replace the regex with manual parsing. **Allocations** come almost entirely from `strings.FieldsFunc` (a new `[]string` per line) and `Scanner.Text` (a new `string` per line). Switching to `Scanner.Bytes()` and indexing fields in place removes ~96% of allocations. The two profiles point at *different* fixes, which is why you collect both.

## Idioms & Best Practices

- **Measure first, optimize second.** Profile the real workload, not your guess. Premature optimization wastes time.
- **Use `testing.B.Loop`** (Go 1.24) for new benchmarks. It's simpler and more idiomatic than the `i := 0; i < b.N` loop.
- **Report allocations:** always include `b.ReportAllocs()` in benchmarks so you can see allocs/op.
- **Preallocation is your friend.** `make([]T, 0, n)` and `make(map[K]V, hint)` avoid costly growth during loops.
- **Hoist expensive operations.** Compile regexes and build lookup tables once at startup or in `var` statements,
not per call or per loop iteration.
- **Use `strings.Builder` for repeated string building.** Grow it to the exact size before writing to avoid
multiple allocations.
- **Reuse buffers in steady-state code.** When the same goroutine processes multiple records, keep a buffer in the
receiver struct and reset it between calls.
- **Choose the right primitive.** `strconv.Append*` over `fmt.Sprintf`, manual parsing over regex when the format is simple,
`[]byte` over `string` when building, `sync.Pool` for expensive-to-allocate types (but use it carefully: see §8).
- **Bounds-check elimination** is automatic: if the compiler can prove an index is safe, it skips the check. Don't manually unroll loops.
- **Data structures matter.** A linked list is O(n) for a random access that a slice does in O(1). A hash table for a small set
of < 20 items is slower than a linear search of a slice (linear search is cache-friendly).

## Common Pitfalls

- **"My benchmark is faster than it should be."** Dead-code elimination (DCE) can optimize away your entire benchmark
if the result isn't used. Modern benchmarks use `testing.B.Loop`, which prevents this. With the old style,
use a package-level `var sink` and assign the result to it: `sink = myFunc()`.
- **"My benchmark varies a lot."** Run with `-count=10` or `-count=100` so `benchstat` can calculate statistics.
A single run is noise. Run on an unloaded machine.
- **"I hit an allocation gate, but I don't know how to optimize further."** Profile with pprof and
look at call sites. The hint in the test message often points you in the right direction. If stuck,
compare your code to the solution's fast.go variant.
- **"My regex is correct but slow."** If the pattern is simple (e.g., digits only), use `strings.Contains`,
`strings.Index` or manual byte scanning. `regexp` is powerful but has overhead. The Go authors often say,
"Regex is fast, but manual parsing is faster."
- **Readable vs. optimized.** Sometimes the readable version is fast enough. A 2x speedup on code that runs
once per second is pointless. Optimize code that runs 1000 times per second or more. Use a profile to decide.
- **Sync.Pool misuse.** `sync.Pool` caches heap-allocated objects to reduce GC pressure, but it only works
when the same goroutine reuses the pool (or you're lucky). For cross-goroutine reuse, a regular buffer in a struct
is more predictable. Keep Pool items small and zeroed.

## Exercises

### 1. appendints

**What to implement:**
- Optimize `AppendInts(dst []byte, nums []int, sep byte) []byte` to be allocation-free when dst has spare capacity.

**The slow version** uses `fmt.Sprintf` to format each integer, which boxes the number into `interface{}`,
allocates a string, and then copies it into dst. The fast version uses `strconv.AppendInt`, which writes digits
straight into the slice.

**Run the test:**
```bash
go test ./modules/09-performance/exercises/appendints/ -v
```

The test fails with: `AppendInts allocates X times per call, want 0.`
It should pass with zero allocations when the buffer is large enough.

---

### 2. csvline

**What to implement:**
- Optimize `Line(fields []string) string` to allocate at most once, instead of once per field.

**The slow version** uses `s +=` in a loop, which is O(n²) in the output size. Each operation allocates a new string.

**Run the test:**
```bash
go test ./modules/09-performance/exercises/csvline/ -v
```

The test fails with: `Line(40 fields) allocates X times, want <= 1.`

**Hint:** Use `strings.Builder`. Call `Grow(estimatedLen)` first to preallocate the exact size.

---

### 3. dedupe

**What to implement:**
- Optimize `Unique(items []string) []string` to cut allocations from ~10+ to ≤2.

**The slow version** grows both the map and the result slice with no hint, so each growth allocates and rehashes/copies.

**Run the test:**
```bash
go test ./modules/09-performance/exercises/dedupe/ -v
```

The test fails with: `Unique allocates X times, want <= 12.` (The exact threshold depends on the input size.)

**Hint:** Preallocate `make(map[K]V, hint)` and `make([]T, 0, capacity)` using `len(items)` as the bound.

---

### 4. lineenc

**What to implement:**
- Optimize `Encoder.Encode(r Record) error` to allocate zero times in steady state.

**The slow version** calls `fmt.Sprintf`, which allocates a string, then converts it to `[]byte`, allocating again.
Every record triggers two allocations.

**Run the test:**
```bash
go test ./modules/09-performance/exercises/lineenc/ -v
```

The test fails with: `Encode allocates X times after warm-up, want 0.`

**Hint:** Add a `[]byte` buffer to the `Encoder` struct. Reset it with `buf = buf[:0]` and format into it using `strconv.AppendQuote`, `strconv.AppendInt`, etc.

---

### 5. semver

**What to implement:**
- Optimize `Parse(s string) (Version, error)` to be allocation-free for valid input.

**The slow version** compiles a regex on every call and uses `FindStringSubmatch`, which allocates a slice of matches.

**Run the test:**
```bash
go test ./modules/09-performance/exercises/semver/ -v
```

The test fails with: `Parse allocates X times, want 0.` (for valid input)

**Hints:**
- Compile the regex once at package level.
- Better: skip regex altogether. Use `strings.Cut` and `strings.HasPrefix` to parse manually.
- Validate component ranges without allocating.

---

## Project: Optimize the Log Parser

See `project/README.md`.

---

## Submitting for Review

When all exercise tests pass, ask your instructor (Claude) to review `modules/09-performance/`.

---

## Further Reading

- [Testing Package - Benchmarking](https://pkg.go.dev/testing#hdr-Benchmarking)
- [Testing Best Practices - Go blog](https://go.dev/blog/testing-b-loop) (b.Loop intro, Go 1.24)
- [Profiling Go Programs](https://go.dev/blog/profiling-go-programs)
- [Diagnostics Guide](https://go.dev/doc/diagnostics)
- [Effective Go - Profiling and Optimizing](https://go.dev/doc/effective_go#profiling)
- [Benchstat tool](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) (not stdlib, but recommended for comparing runs)
- [100 Go Mistakes - Performance](https://100go.co) (chapters on allocations, regex, preallocation)
