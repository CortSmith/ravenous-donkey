# Module 04 — Testing Like a Gopher

This module turns the course around. Until now you were given tests and wrote
code. Here you get **code with planted bugs and almost no tests**, and your job
is to write the tests that expose the bugs, watch them fail, and only then fix
the code. That is how testing works in real life: nobody hands you the failing
test.

Every package tells you how many bugs it has. The starter tests that ship
with each exercise **pass**. That is deliberate: weak tests pass against
buggy code, and you are about to find out how.

## Goals

After this module you will be able to:

- Write table-driven tests with named subtests, and run a single case from the command line.
- Write failure messages that make a bug obvious without opening the test file.
- Use the `testing` helpers correctly: `t.Helper`, `t.Cleanup`, `t.TempDir`, `t.Setenv`, `t.Context`, `t.Parallel`.
- Choose between `package foo` (white-box) and `package foo_test` (black-box) tests.
- Test code that depends on time and the outside world with **fakes** injected through small interfaces.
- Write golden-file tests with an `-update` flag, and review golden files critically.
- Write fuzz tests with real **properties** (round-trip, oracle, invariants) and turn fuzzer findings into regression tests.
- Write `Example` functions that are documentation *and* tests.
- Write benchmarks with Go 1.24's `b.Loop()`, read `-benchmem` output, and pin allocation promises with `testing.AllocsPerRun`.
- Measure coverage and read a coverage report to find untested branches.

## Concepts

### 1. The mechanics: what `go test` actually does

`go test` compiles each package **together with its `_test.go` files** into a
test binary and runs it. Inside `_test.go` files:

| Function shape | Kind | Runs on plain `go test`? |
|---|---|---|
| `func TestXxx(t *testing.T)` | test | yes |
| `func ExampleXxx()` with `// Output:` | example | yes (output compared) |
| `func ExampleXxx()` without `// Output:` | example | **compiled only** |
| `func BenchmarkXxx(b *testing.B)` | benchmark | only with `-bench` |
| `func FuzzXxx(f *testing.F)` | fuzz test | seeds + corpus only; fuzzes with `-fuzz` |
| `func TestMain(m *testing.M)` | setup hook | wraps everything (rarely needed) |

Useful flags (all commands are run from the course root):

```bash
go test ./modules/04-testing/...               # every package below this dir
go test -v ./modules/04-testing/exercises/paginate/   # verbose: every test, every log
go test -run 'TestBounds/page_zero' ./...      # one subtest (regexp per level, split on /)
go test -count=1 ./...                         # bypass the test result cache
go test -shuffle=on ./...                      # random test order: finds hidden coupling
go test -race ./...                            # data race detector
go test -failfast ./...                        # stop at the first failure
```

`go test` caches passing results. `(cached)` in the output means nothing
was re-run. Use `-count=1` when a test reads files or the environment that
Go can't track.

### 2. Table-driven tests and subtests

The idiomatic Go test is a **table**: a slice of cases, and one loop that runs
each case as a named subtest.

```go
func TestPages(t *testing.T) {
	tests := []struct {
		name           string
		total, perPage int
		want           int
	}{
		{"no items", 0, 10, 0},
		{"partial page", 5, 10, 1},
		{"exact multiple", 20, 10, 2},
		{"one over", 21, 10, 3},
		{"bad page size", 10, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Pages(tt.total, tt.perPage); got != tt.want {
				t.Errorf("Pages(%d, %d) = %d, want %d", tt.total, tt.perPage, got, tt.want)
			}
		})
	}
}

// Pages returns how many pages of perPage items hold total items.
func Pages(total, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	return (total + perPage - 1) / perPage
}
```

Why this shape wins:

- **Adding a case is one line.** Edge cases stop being "too much effort".
- **Subtests are independent.** A `t.Fatal` in one case stops only that case.
- **Each case has a name**, so `go test -run 'TestPages/exact_multiple'` runs
  just that one. Spaces in names become underscores.
- Since **Go 1.22** each loop iteration has its own `tt`, so the old
  `tt := tt` line is no longer needed, even with `t.Parallel()`
  ([Go blog: Fixing For Loops in Go 1.22](https://go.dev/blog/loopvar-preview)).

Use **named fields** in the table (`{name: "x", line: "...", wantErr: ErrSyntax}`)
once a case has more than three or four fields, or when most cases leave some
fields at their zero value.

#### `t.Error` vs `t.Fatal`

- `t.Errorf` records a failure and **keeps going**: you see every wrong field
  at once.
- `t.Fatalf` records a failure and **stops this test or subtest** (via
  `runtime.Goexit`). Use it when continuing makes no sense, e.g. the call
  returned an error so the result is meaningless.
- Call `t.Fatal*` only from the goroutine running the test. From another
  goroutine, it cannot stop the test. Send the error back on a channel instead.

#### Failure messages

The [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments#useful-test-failures)
convention: **function, input, got, want**, in that order.

```go expect=runtime-error
// This test fails on purpose, so you can compare the messages.
func TestMessages(t *testing.T) {
	line, got, want := "a,b", []string{"a"}, []string{"a", "b"}

	// Good: tells you everything without opening the file.
	t.Errorf("Split(%q) = %q, want %q", line, got, want)

	// Bad: which input? which value is which?
	t.Errorf("wrong result")
	t.Errorf("expected %v but got %v", want, got) // backwards, no input
}
```

Use `%q` for strings: it makes empty strings, spaces, quotes and newlines
visible. A failing message like `Join(["a" ""]) = "a", want "a,"` is half the
debugging done for you.

#### Comparing values

- Comparable values: `==`.
- Slices: `slices.Equal(got, want)`; maps: `maps.Equal(got, want)`.
- Structs containing slices or maps: `reflect.DeepEqual`. It works, but its
  failure output is poor, so print both sides with `%+v`. (Outside the standard
  library, `github.com/google/go-cmp/cmp.Diff` is the usual choice.)
- Errors: **`errors.Is(err, ErrX)`**, never `err == ErrX` (breaks when
  wrapped) and never `err.Error() == "..."` (breaks when reworded).

### 3. Helpers and lifecycle

A **helper** is a function your tests call that itself reports failures. Mark
it with `t.Helper()` so failures point at the *caller's* line:

```go
func assertPages(t *testing.T, total, perPage, want int) {
	t.Helper() // without this, every failure points at the line below
	if got := Pages(total, perPage); got != want {
		t.Errorf("Pages(%d, %d) = %d, want %d", total, perPage, got, want)
	}
}

func TestPagesHelper(t *testing.T) {
	assertPages(t, 21, 10, 3)
	assertPages(t, 0, 10, 0)
	// Change a want above: the failure then names this line, not the helper's.
}

// Pages returns how many pages of perPage items hold total items.
func Pages(total, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	return (total + perPage - 1) / perPage
}
```

The `testing.T` toolbox for setup and teardown:

```go
func TestImport(t *testing.T) {
	dir := t.TempDir()           // fresh dir, removed after the test
	t.Setenv("APP_MODE", "test") // restored after the test
	ctx := t.Context()           // Go 1.24: cancelled just before Cleanup funcs run
	db := openTestDB(t, dir)     // a helper that registers its own cleanup
	t.Logf("db at %s, mode %s, ctx err %v", db.path, os.Getenv("APP_MODE"), ctx.Err())
}

func openTestDB(t *testing.T, dir string) *DB {
	t.Helper()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() }) // runs when the *test* ends, not when this helper returns
	return db
}

// A stand-in database: just a file.
type DB struct {
	path string
	f    *os.File
}

func Open(path string) (*DB, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &DB{path: path, f: f}, nil
}

func (db *DB) Close() error { return db.f.Close() }
```

Why `t.Cleanup` instead of `defer`? A `defer` inside `openTestDB` would run
when the **helper** returns, closing the database before the test uses it.
`t.Cleanup` runs when the **test** (and all its subtests) finishes, in
last-in-first-out order. Inside the test function itself, `defer` and
`t.Cleanup` both work; in helpers only `t.Cleanup` does.

(`t.Context` and `t.Chdir` are new in Go 1.24.)

### 4. Parallel tests

`t.Parallel()` marks a test or subtest as safe to run alongside other parallel
tests. It pays off for slow, independent cases. For microsecond-fast table
cases, the gain is negligible.

```go
func TestFetch(t *testing.T) {
	srv := startFakeServer(t) // shared, read-only fixture
	t.Cleanup(srv.Close)      // NOT defer, see below
	tests := []struct{ name, path, want string }{
		{"root", "/", "hello /"},
		{"page", "/about", "hello /about"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := http.Get(srv.URL + tt.path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if got := string(body); got != tt.want {
				t.Errorf("GET %s = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func startFakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello %s", r.URL.Path)
	}))
}
```

The trap: when a subtest calls `t.Parallel()`, it **pauses until the parent
test function returns**. So `defer srv.Close()` in the parent would run
*before* the parallel subtests do. `t.Cleanup` waits for the subtests.

Don't parallelise tests that share mutable state (package variables, the
working directory, environment variables). `t.Setenv` and `t.Chdir` panic in
parallel tests for exactly this reason. Run `go test -race` on anything
parallel.

### 5. White-box vs black-box tests

A `_test.go` file in directory `semver/` can declare either package:

- `package semver`: **white-box**. Sees unexported identifiers. Good for
  testing a tricky private helper (`money`, `checkIdents`).
- `package semver_test`: **black-box**. Must import the package like any
  user would, and sees only the exported API:

```go
package semver_test

import (
	"testing"

	"gocourse/modules/04-testing/project/semver"
)

func TestParse(t *testing.T) {
	v, err := semver.Parse("1.2.3")
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", "1.2.3", err)
	}
	if got := v.String(); got != "1.2.3" {
		t.Errorf("Parse(%q).String() = %q, want %q", "1.2.3", got, "1.2.3")
	}
	// v.checkIdents(...) would not compile here: semver_test sees only the exported API.
}
```

Prefer black-box tests for a package's public behaviour: they survive
refactoring, and they read like documentation. `Example` functions are almost
always black-box. Both kinds can live in the same directory.

### 6. Fakes: testing code that touches time and the world

Code that calls `time.Now()` or sends real email directly is hard to test.
The fix is the same one you met in module 03: depend on a **small interface**,
and pass a **fake** in tests.

```go
// production code
type Clock interface{ Now() time.Time }

// test code
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

var _ Clock = (*fakeClock)(nil) // compile-time check: fakeClock implements Clock

func TestFakeClock(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)}
	var c Clock = clk // production code only sees the interface
	clk.Advance(90 * time.Minute)
	if got := c.Now().Format("15:04"); got != "10:30" {
		t.Errorf("Now() = %s, want 10:30", got)
	}
}
```

A **fake** is a tiny working implementation: it records what happened, and the
test inspects its state afterwards. A **mock** is scripted with expected
calls in advance. Go code mostly prefers fakes: they are simpler, and tests
check *outcomes* rather than the exact sequence of calls, so they break less
often when the implementation changes.

Rules of thumb:

- **Never `time.Sleep` in a test** to "wait until something is due". Advance
  the fake clock instead. Tests become instant and deterministic.
- Make the fake able to **fail** (e.g. a map of recipient → error) so you can
  test error paths.
- Build fresh fakes in every test (a `newTestX(t)` helper), never share them.

(Go 1.25 adds `testing/synctest`, which runs goroutines in a bubble with a fake
clock. It is out of scope here, since this course targets Go 1.24, but worth
knowing about.)

### 7. Golden files

For large outputs (rendered tables, generated code, JSON), writing the
expected string inline is painful. A **golden file** stores the expected
output under `testdata/`. The `go` tool ignores `testdata` directories when
building, so they are the conventional place for test inputs and outputs.

```go
var update = flag.Bool("update", false, "rewrite golden files")

func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s:\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

func TestReport(t *testing.T) {
	got := []byte("total: 3 items\n")
	path := filepath.Join(t.TempDir(), "report.golden") // normally testdata/report.golden
	os.WriteFile(path, []byte("total: 3 items\n"), 0o644) // as if a reviewed -update run made it
	assertGolden(t, path, got)
}
```

The workflow:

1. Write the test. Run it with `-update` to create the files:
   `go test ./modules/04-testing/exercises/report/ -update`
2. **Open every golden file and check it by eye against the spec.** This is
   the step people skip, and it is the whole point.
3. Run without `-update`: from now on, any change in output fails the test.

The danger is that **`-update` records whatever the code produces, bugs
included.** If you run it on buggy code and don't read the result, your test
now *guarantees* the bug. Pair golden tests with a few small, targeted
assertions (a unit test for a formatting helper, a check of one specific
line) that do not depend on the golden file.

### 8. Example functions

```go
// This example uses the reference semver. Point the import at
// gocourse/modules/04-testing/project/semver to check yours.
import "gocourse/solutions/04-testing/project/semver"

func ExampleCompare() {
	a := semver.MustParse("1.0.0-rc.1")
	b := semver.MustParse("1.0.0")
	fmt.Println(semver.Compare(a, b))
	// Output: -1
}
```

- `go test` runs the example and compares its stdout with the `// Output:`
  comment (leading and trailing whitespace ignored). It fails if they differ.
- **No `// Output:` comment → the example is compiled but never run.**
- `// Unordered output:` compares lines in any order (for map iteration).
- Naming links the example to an identifier in `go doc` and pkg.go.dev:
  `Example` (package), `ExampleCompare` (function), `ExampleVersion_String`
  (method), `ExampleCompare_prerelease` (a second example; the suffix starts
  lowercase).

Examples are the best documentation you can write, because they cannot go
stale: they are tested.

### 9. Fuzzing

A table test checks the inputs **you** thought of. A fuzz test states a
**property** that must hold for *every* input, and lets the fuzzer search for
an input that breaks it ([Go fuzzing docs](https://go.dev/doc/security/fuzz)).

```go
func FuzzReverse(f *testing.F) {
	f.Add("hello") // seed corpus: ordinary examples
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			t.Skip() // property only defined for valid UTF-8
		}
		if got := Reverse(Reverse(s)); got != s {
			t.Errorf("Reverse(Reverse(%q)) = %q", s, got)
		}
	})
}

// Reverse reverses s rune by rune.
func Reverse(s string) string {
	r := []rune(s)
	slices.Reverse(r)
	return string(r)
}
```

```bash
go test ./pkg/                                     # runs seeds + testdata corpus only (fast)
go test ./pkg/ -run='^$' -fuzz='^FuzzReverse$' -fuzztime=20s   # actually fuzz
```

What you need to know:

- `-fuzz` works on **one package and one fuzz target** at a time; the regexp
  must match exactly one `FuzzXxx`. `-run='^$'` skips the normal tests.
- Fuzz arguments may only be: `string`, `[]byte`, `bool`, `float32`,
  `float64`, and the integer types (`int`, `int8`…`int64`, `uint`,
  `uint8`…`uint64`, plus `byte` and `rune`). Build structs inside the fuzz
  function from these.
- When the fuzzer finds a failure, it **minimises** the input and writes it to
  `testdata/fuzz/FuzzXxx/<hash>`. From then on, plain `go test` replays it as
  a regression test. **Commit these files.** Then copy the input into a
  table test with a `// Found by FuzzXxx` comment so humans can read it.
- A panic inside the target counts as a failure. You don't need to `recover`.

Good fuzz properties, from strongest to weakest:

1. **Round-trip**: `Decode(Encode(x)) == x`; `Parse(v.String()) == v`.
2. **Oracle / differential**: the result equals a slow-but-obviously-correct
   implementation (`strings.Join`, the readable version of a fast function).
3. **Invariants**: output is sorted; `Compare(a, b) == -Compare(b, a)`;
   errors are always one of the documented sentinels.
4. **Doesn't panic**: the weakest property, but still finds real bugs.

A fuzz target that checks nothing except "doesn't panic" will miss every bug
that returns a wrong answer quietly.

### 10. Benchmarks

```go
func BenchmarkJoin(b *testing.B) {
	elems := makeElems(100, 8) // setup: runs once, not timed
	b.ReportAllocs()
	for b.Loop() {
		strings.Join(elems, ",")
	}
}

// makeElems returns n strings of the given length.
func makeElems(n, size int) []string {
	elems := make([]string, n)
	for i := range elems {
		elems[i] = strings.Repeat("x", size)
	}
	return elems
}
```

```bash
go test ./pkg/ -run='^$' -bench=. -benchmem -count=5
```

`for b.Loop()` (new in Go 1.24, see the [Go blog post](https://go.dev/blog/testing-b-loop))
replaces the old `for i := 0; i < b.N; i++`:

- The benchmark function runs **once** per `-count`, so setup before the loop
  runs once too. You no longer need `b.ResetTimer()`: the timer starts at the
  first `b.Loop()` call.
- Calls in the loop body are **kept alive**: the compiler cannot optimise
  away `strings.Join(...)` just because the result is unused. No more
  `var sink string` tricks.

Reading the output:

```
BenchmarkJoin/Builder/n=100-20   790906   1511 ns/op   1024 B/op   1 allocs/op
```

That line shows: the name with sub-benchmark and `GOMAXPROCS`, the iterations
run, time per op, bytes allocated per op, and allocations per op.
**Allocations are often the real cost:** each one costs time now and
garbage-collector work later.

- Use `b.Run("name", ...)` for sub-benchmarks (sizes, implementations).
- Benchmarks are noisy. Run `-count=5` or more, and compare runs with
  `benchstat` (from `golang.org/x/perf`, an external tool):
  `benchstat old.txt new.txt`.
- **Pin performance promises in tests.** If a doc comment says "allocates
  once", enforce it in a test that runs on every `go test`:

```go
func TestJoinBuilderAllocatesOnce(t *testing.T) {
	elems := makeElems(100, 10)
	if n := testing.AllocsPerRun(100, func() { JoinBuilder(elems, ",") }); n > 1 {
		t.Errorf("JoinBuilder allocated %.1f times per call, want at most 1", n)
	}
}

// JoinBuilder is strings.Join with one up-front allocation.
func JoinBuilder(elems []string, sep string) string {
	n := len(sep) * max(len(elems)-1, 0)
	for _, e := range elems {
		n += len(e)
	}
	var b strings.Builder
	b.Grow(n)
	for i, e := range elems {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(e)
	}
	return b.String()
}

func makeElems(n, size int) []string {
	elems := make([]string, n)
	for i := range elems {
		elems[i] = strings.Repeat("x", size)
	}
	return elems
}
```

### 11. Coverage

```bash
go test -cover ./modules/04-testing/project/semver/
go test -coverprofile=c.out ./modules/04-testing/project/semver/
go tool cover -func=c.out    # percentage per function
go tool cover -html=c.out    # source view: green = run, red = never run
```

Coverage tells you which lines **never ran**. It cannot tell you whether the
lines that ran were *checked*. A test that calls every function and asserts
nothing has 100% coverage and catches nothing. Use the HTML view to find
untested branches, then ask "what input reaches that branch, and what should
happen?". Aim for high coverage of logic; chasing the last few percent on
trivial code is rarely worth it.

## Idioms & Best Practices

- **Table-driven + `t.Run`** for anything with more than two cases.
  ```go
  // Good: one table, one loop, named subtests
  func TestPages(t *testing.T) {
  	tests := []struct {
  		name               string
  		total, per, want int
  	}{
  		{"partial", 5, 10, 1},
  		{"one over", 21, 10, 3},
  	}
  	for _, tt := range tests {
  		t.Run(tt.name, func(t *testing.T) {
  			if got := Pages(tt.total, tt.per); got != tt.want {
  				t.Errorf("Pages(%d, %d) = %d, want %d", tt.total, tt.per, got, tt.want)
  			}
  		})
  	}
  }

  // Bad: one function per case, copy-pasted
  func TestPages1(t *testing.T) {
  	if Pages(5, 10) != 1 {
  		t.Error("fail")
  	}
  }
  func TestPages2(t *testing.T) {
  	if Pages(21, 10) != 3 {
  		t.Error("fail")
  	}
  }

  // Pages returns how many pages of perPage items hold total items.
  func Pages(total, perPage int) int {
  	if perPage <= 0 {
  		return 0
  	}
  	return (total + perPage - 1) / perPage
  }
  ```
- **Function, input, got, want** in every failure message.
  ```go expect=runtime-error
  // Fails on purpose: compare the two messages.
  func TestMessage(t *testing.T) {
  	total, per, want := 21, 10, 3
  	got := 2
  	t.Errorf("Pages(%d, %d) = %d, want %d", total, per, got, want) // good
  	t.Errorf("got %d", got)                                        // bad
  }
  ```
- **`t.Helper()` first line** of every helper that calls `t.Error*`/`t.Fatal*`.
- **`t.Cleanup` in helpers**, never `defer` (a deferred close in a helper runs too early).
- **`errors.Is` / `errors.As`** for errors.
  ```go
  var ErrInvalidPage = errors.New("page out of range")

  err := fmt.Errorf("bounds(total=5, page=3): %w", ErrInvalidPage)
  fmt.Println(errors.Is(err, ErrInvalidPage))     // true: good
  fmt.Println(err.Error() == "page out of range") // false: bad, breaks on rewording or wrapping
  ```
- **Inject dependencies** (clock, sender, filesystem via `fs.FS`, `io.Writer`) instead of reaching for globals.
  ```go
  type Clock interface{ Now() time.Time }
  type Sender interface{ Send(to, body string) error }

  type Scheduler struct {
  	clock  Clock
  	sender Sender
  	due    time.Time
  }

  func New(clock Clock, sender Sender) *Scheduler { return &Scheduler{clock: clock, sender: sender} }

  // Good: the time comes from the injected clock, so a test controls it.
  func (s *Scheduler) Flush() error {
  	if s.clock.Now().Before(s.due) {
  		return nil
  	}
  	return s.sender.Send("ada", "time's up")
  }

  // Bad would be `if time.Now().Before(s.due)`: an untestable boundary.

  // Fakes for a test:
  type fixedClock time.Time

  func (c fixedClock) Now() time.Time { return time.Time(c) }

  type printSender struct{}

  func (printSender) Send(to, body string) error { fmt.Println("to", to+":", body); return nil }

  start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
  s := New(fixedClock(start), printSender{}) // good: test passes fakes
  s.due = start.Add(time.Hour)
  s.Flush() // not due: prints nothing
  s.clock = fixedClock(start.Add(2 * time.Hour))
  s.Flush() // to ada: time's up
  ```
- **Fakes over mocks**: assert on outcomes, not on call sequences.
- **Test against an oracle** when one exists: `strings.Join`, the spec's example list, a slow reference implementation.
- **`testdata/`** for fixtures and golden files; never write test output next to source files.
- **Fuzz anything that parses.** A round-trip property is usually one line.
- **Commit fuzz findings** (`testdata/fuzz/...`) and copy them into a readable table case.
- **`b.Loop()`, `b.ReportAllocs()`**, setup outside the loop.
- **Write the failing test first** when fixing a bug: it proves the bug exists and that your fix fixed it.

## Common Pitfalls

- **Tests that cannot fail.** The test passes against the buggy code too.
  The only way to know is to see it fail: write the test *before* the fix and
  watch it go red. If you fixed first, temporarily put the bug back.
- **Testing the implementation instead of the spec.** If you derive the
  expected values by reading the code, you copy its bugs into your tests.
  Derive them from the doc comment or the spec.
- **Missing boundaries.** Off-by-one bugs live at `0`, `1`, `n-1`, `n`,
  `n+1`, at "exactly equal" times, and in empty inputs. Test both sides of
  every boundary.
- **Blind `-update`.** Regenerating golden files on buggy code and committing
  them unread turns a bug into a test requirement.
- **Byte length vs rune count.** `len("é") == 2`. Text layout tests need
  non-ASCII input.
- **`defer` in a test helper**, or in a parent test with parallel subtests:
  it runs too early. Use `t.Cleanup`.
- **`t.Fatal` from a spawned goroutine**: it can't stop the test. Report
  through a channel, or use `t.Error`.
- **Shared state between tests**: package-level fakes, environment variables,
  the working directory. `go test -shuffle=on` and `-count=2` flush these out.
- **`time.Sleep` in tests**: slow and flaky. Inject a clock.
- **An `Example` without `// Output:`** silently never runs.
- **A fuzz target with no property** ("call it and see if it panics") only
  finds crashes, never wrong answers.
- **Benchmarking a wrong function.** A fast wrong answer is worthless. Make
  the correctness test pass before you look at benchmark numbers.
- **Trusting one benchmark run.** Noise of ±10% is normal on a laptop. Use
  `-count` and `benchstat`.
- **Running the fuzzer on several targets at once.** `-fuzz=Fuzz` matches
  both `FuzzRoundTrip` and `FuzzSplit` and is rejected; anchor the regexp.

## Exercises

General workflow for every exercise:

1. Read the package's doc comments: they are the spec.
2. Run the starter tests. They pass, which is the problem.
3. Write tests that pin down the documented behaviour. Run them. They should
   fail; each failure points at a bug.
4. Only then fix the code, with the smallest change that makes the test pass.
5. Ask yourself: "would my tests fail if someone re-introduced this bug?"

Bug counts are given; the bugs themselves are for you to find.

### 1. csvline — fuzzing (2 bugs)

`csvline` encodes a list of fields as one CSV line (`Join`) and decodes it
again (`Split`). The two bugs are hard to find with hand-written examples.
Write:

- `TestJoin`, `TestSplit`: table-driven, including error cases checked with `errors.Is`.
- `FuzzRoundTrip`: the property is that `Split` undoes `Join`.
- `FuzzSplit`: arbitrary input must never panic, errors must be the documented sentinels, and whatever is accepted must round-trip.

When the fuzzer finds something, read the failing input, add it to your table
test with a `// Found by FuzzXxx` comment, then fix the code. Keep the files
the fuzzer writes under `testdata/fuzz/`.

```bash
go test ./modules/04-testing/exercises/csvline/
go test ./modules/04-testing/exercises/csvline/ -run='^$' -fuzz='^FuzzRoundTrip$' -fuzztime=20s
go test ./modules/04-testing/exercises/csvline/ -run='^$' -fuzz='^FuzzSplit$' -fuzztime=20s
```

### 2. paginate — table tests and boundaries (3 bugs)

`Pages(total, perPage)` counts pages; `Bounds(total, perPage, page)` returns
the half-open index range `[start, end)` shown on a page. Write:

- `TestPages` and `TestBounds`: table-driven, covering both sides of every
  boundary and every documented error.
- Bonus `TestBoundsCoverEveryItem`: a **property test** without the fuzzer. For
  every `total` in 0..40 and `perPage` in 1..12, walk all pages and check
  they cover `[0, total)` exactly once, with no gaps or overlaps.
- Bonus `ExampleBounds`.

```bash
go test ./modules/04-testing/exercises/paginate/
```

### 3. concat — oracles, allocations, benchmarks (2 bugs)

Three ways to join strings, each promising to equal `strings.Join`.
`JoinBuilder` also promises to allocate once. Write:

- `TestJoinersMatchStringsJoin`: one table of inputs, run against every
  joiner, with `strings.Join` as the **oracle**.
- `TestJoinBuilderAllocatesOnce`: pin the promise with `testing.AllocsPerRun`.
- `BenchmarkJoin`: every joiner at 10, 100 and 1000 elements, with
  `b.Run`, `b.Loop`, `b.ReportAllocs`.

Fix correctness first, then read the benchmark. Can you explain *why* the
fastest one is faster, using the `allocs/op` column?

```bash
go test ./modules/04-testing/exercises/concat/
go test ./modules/04-testing/exercises/concat/ -run='^$' -bench=. -benchmem
```

### 4. reminder — fakes and an injected clock (3 bugs)

A `Scheduler` sends reminders through a `Sender` once a `Clock` says they are
due. Write a `fakeClock` (with `Advance`) and a `fakeSender` (records sends,
can fail for chosen recipients), then:

- `TestFlushSendsOnlyDueReminders`: what exactly counts as "due"?
- `TestFlushLaterAfterClockAdvances`
- `TestFlushSendsEarliestFirst`
- `TestFlushKeepsFailedRemindersForRetry`
- `TestFlushJoinsAllErrors`

No `time.Sleep` anywhere.

```bash
go test ./modules/04-testing/exercises/reminder/
```

### 5. report — golden files (3 bugs)

`Render` prints a bill as a text table. Write:

- `TestRenderGolden`: table-driven, one golden file per case under
  `testdata/`, with an `-update` flag. Choose cases that vary what real bills
  vary.
- `TestMoney`: a white-box unit test for the unexported `money` helper.
- Bonus: a test of the layout rules that doesn't use golden files, and
  `ExampleRender`.

**Read every golden file** against `Render`'s doc comment before you trust
it. If `-update` wrote down a bug, your test now protects the bug.

```bash
go test ./modules/04-testing/exercises/report/ -update   # write golden files
# now READ modules/04-testing/exercises/report/testdata/*.golden
go test ./modules/04-testing/exercises/report/
```

## Project

**Semantic Versioning (≤ 60 min).** `project/semver` is a complete
SemVer 2.0.0 parser and comparator with **4 planted bugs** and no tests. Write
a black-box test suite from the [specification](https://semver.org/spec/v2.0.0.html):
table tests, the spec's own precedence list, fuzz tests, examples and
benchmarks. Reach ≥ 90% coverage, then fix the bugs. Full requirements are in
[project/README.md](project/README.md).

```bash
go test ./modules/04-testing/project/semver/
go test -cover ./modules/04-testing/project/semver/
```

## Submitting for Review

When all your tests pass, ask your instructor (Claude) to review
`modules/04-testing/`. Expect the review to **put each bug back, one at a
time, and run your tests**. A bug your tests don't catch is a required
change, even if you fixed it in the code.

## Further Reading

- [`testing` package documentation](https://pkg.go.dev/testing): `T`, `B`, `F`, examples, `AllocsPerRun`.
- [Go wiki: Table-driven tests](https://go.dev/wiki/TableDrivenTests)
- [Go Code Review Comments: Useful test failures](https://go.dev/wiki/CodeReviewComments#useful-test-failures)
- [Go blog: Using Subtests and Sub-benchmarks](https://go.dev/blog/subtests)
- [Go fuzzing documentation](https://go.dev/doc/security/fuzz) and [tutorial](https://go.dev/doc/tutorial/fuzz)
- [Go blog: More predictable benchmarking with `testing.B.Loop`](https://go.dev/blog/testing-b-loop)
- [Go blog: The cover story](https://go.dev/blog/cover): how coverage works.
- [Go blog: Testable Examples in Go](https://go.dev/blog/examples)
- [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat): comparing benchmark runs.
- Mitchell Hashimoto, *Advanced Testing with Go* (GopherCon 2017 talk): fakes, golden files, test helpers in practice.
