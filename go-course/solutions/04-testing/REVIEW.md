# Module 04 — Testing: Instructor Review Guide

This module is inverted compared with the others: the **code is given, the
tests are the student's work**. Every starter package contains planted bugs.
The student writes tests that expose them, watches the tests fail, then fixes
the code. So a review grades two things, in this order:

1. **Do the student's tests catch every planted bug?** (Most important. A
   fixed `.go` file proves nothing: the student may have spotted the bug by
   reading.)
2. Is the test code idiomatic, and is the fix correct and minimal?

All numbers in this file were measured on **go1.24.2 linux/amd64**
(20 logical CPUs), `-count=5`, and are rounded. Absolute times will differ on
other machines; the ratios and allocation counts are what matter.

---

## How to grade: re-plant the bugs (mutation check)

The student has fixed the bugs, so their tests pass. To prove the tests
*would have* caught each bug, put the bugs back **one at a time** and run the
student's tests. This is mutation testing done by hand.

```bash
# from the course root; repeat for each package
W=$(mktemp -d)
cp -r modules/04-testing/exercises/paginate/. "$W"
printf 'module gocourse/modules/04-testing/exercises/paginate\n\ngo 1.24\n' > "$W/go.mod"
cd "$W"
# re-plant ONE bug from the table below, e.g.:
sed -i 's/if page < 1 ||/if page < 0 ||/' paginate.go
go test ./...      # must FAIL; note which test caught it
```

For semver the student's test is a black-box test (`package semver_test`)
importing `gocourse/modules/04-testing/project/semver`, which is why the
`go.mod` above names the module path rather than an arbitrary one.

A bug the student's suite does not catch is a **required change**, even if
their `.go` fix is correct.

Each bug below was verified this way against the reference tests: every one,
applied alone to the reference solution, makes `go test` fail, and the
"caught by" column lists the tests that failed.

---

## Bug inventory (17 bugs)

| Package | # | Bug (one-line mutation of the solution) | Caught by (reference tests) |
|---|---|---|---|
| csvline | 1 | `Join`: quote test omits `"` — `ContainsAny(f, ",\r\n")` | `TestJoin/{quote_inside,leading_quote,just_a_quote}`, `FuzzRoundTrip` |
| csvline | 2 | `Split`: error snippet `line[i:i+8]` panics on short input | `TestSplit/short_junk_after_closing_quote`, `FuzzSplit` |
| concat | 1 | `JoinBuilder` skips empty elements (`if e == "" { continue }`) | `TestJoinersMatchStringsJoin/Builder/{empty_middle,empty_last,all_empty}`, `ExampleJoinBuilder` |
| concat | 2 | `JoinBuilder` under-sizes `Grow` (`len(elems)*len(sep)`) | `TestJoinBuilderAllocatesOnce` (5 allocs vs ≤1) |
| paginate | 1 | `Pages` returns `total/perPage + 1` | `TestPages/{no_items,exactly_one_page,exact_multiple,one_item_per_page}`, `TestBounds/page_after_exact_multiple`, `TestBoundsCoverEveryItem` |
| paginate | 2 | `Bounds` accepts page 0 (`page < 0`) | `TestBounds/page_zero`, `TestBoundsCoverEveryItem` |
| paginate | 3 | `Bounds` doesn't clamp `end` to `total` | `TestBounds/{last_partial_page,fewer_items_than_a_page}`, `TestBoundsCoverEveryItem`, `ExampleBounds` |
| report | 1 | total adds unit price, not `Qty*Price` | `TestRenderGolden/{basic,cents,unicode,wide_numbers}`, `TestRenderLayoutRules`, `ExampleRender` |
| report | 2 | column width uses `len` (bytes), not runes | `TestRenderGolden/unicode`, `TestRenderLayoutRules` |
| report | 3 | `money` uses `%d.%d` (5¢ → `0.5`) | `TestMoney`, `TestRenderGolden/*`, `TestRenderLayoutRules`, `ExampleRender` |
| reminder | 1 | `Flush` doesn't sort by due time | `TestFlushSendsEarliestFirst` |
| reminder | 2 | due test `!Due.Before(now)` — a reminder due exactly now is not sent | `TestFlushSendsOnlyDueReminders/exactly_now`, `TestFlushLaterAfterClockAdvances` |
| reminder | 3 | failed sends are dropped instead of kept for retry | `TestFlushKeepsFailedRemindersForRetry` |
| semver | 1 | MAJOR/MINOR/PATCH leading zeros accepted (`01.2.3`) | `TestParseInvalid/{01.2.3,1.02.3,1.2.00}` |
| semver | 2 | empty identifiers accepted; `1.2.3-` **panics** (`isNumeric("")` is true, then `id[0]`) | `TestParseInvalid/{1.2.3-,1.2.3-a.,1.2.3-a..b,1.2.3+,1.2.3+a..b}` |
| semver | 3 | a release does not outrank its pre-releases (`1.0.0-rc.1 < 1.0.0` fails) | `TestCompare/1.0.0-rc.1_vs_1.0.0`, `TestCompareSpecOrder`, `TestSort`, `ExampleCompare`, `ExampleSort` |
| semver | 4 | numeric pre-release ids compared as strings (`alpha.10 < alpha.2`) | `TestCompare/1.0.0-alpha.2_vs_1.0.0-alpha.10`, `TestCompare/…99999999999999999999…`, `TestCompareSpecOrder`, `TestSort` |

Notes on the inventory:

- **report bug 2** needs two edits to re-plant (drop the `unicode/utf8` import
  and use `len(cell)`), otherwise the file does not compile.
- **semver bug 2** spans two functions in the starter: `checkIdents` lost its
  `id == ""` check and `isNumeric` lost its `s == ""` check. Restoring only
  `isNumeric`'s check is not observable through the public API; restoring
  `checkIdents`'s check fixes the behaviour. Count it as one bug. A fix that
  restores only one of the two is acceptable if the tests pass, but mention the
  other in review.
- Fuzzing: with an **empty corpus**, `FuzzRoundTrip` finds csvline bug 1 and
  `FuzzSplit` finds csvline bug 2 in well under a second. If a student says
  "the fuzzer found nothing", their fuzz target is wrong (usually it checks
  nothing, or it ignores the error from `Split`).
- Bugs mask each other. For example, with paginate bug 1 present,
  `Bounds(20, 10, 3)` is accepted because `Pages` says there are 3 pages.
  The student may see a test fail for a "different" reason than expected.
  That is fine; what matters is that each bug *alone* is caught.

---

## Exercise 1 — csvline (fuzzing)

### What a good student suite looks like
- `TestJoin` / `TestSplit` tables including: `nil`, a single empty field,
  empty fields at the edges (`",x,"`), comma / CR / LF / quote inside a
  field, an escaped quote, an unterminated quote, junk after the closing
  quote (both long and **short**).
- Error cases checked with `errors.Is(err, ErrSyntax)`, not string compare.
- `FuzzRoundTrip` with a **real property** (`Split(Join(x)) == x`) and
  `FuzzSplit` with "never panic; errors are one of the two sentinels; whatever
  is accepted round-trips".
- After fuzzing, the failing inputs are promoted into the table tests with a
  `// Found by FuzzX` comment. The `testdata/fuzz/...` files the fuzzer
  writes are committed too: plain `go test` replays them as regression tests.

### Readable solution
`Join` writes into a `strings.Builder` and quotes a field only when it
contains `,`, `"`, CR or LF, doubling inner quotes with `strings.ReplaceAll`.
`Split` is a single forward scan; quoted fields go through a `strings.Builder`
so escaped quotes can be collapsed. Both fixes are one-liners:
add `"` to the `ContainsAny` set; clamp the snippet with
`line[i:min(i+8, len(line))]`.

### Optimized solution — `fast.go` (`JoinFast`, `SplitFast`)
- `JoinFast` computes the exact output length first (`len` + 2 quotes + one
  extra byte per inner `"`), calls `Grow` once, and writes doubled quotes byte
  by byte, so there are no `ReplaceAll` temporaries.
- `SplitFast` sizes the result slice with `strings.Count(line, ",")+1` (an
  upper bound on the field count) and returns quoted fields that contain no
  escaped quote as **substrings of `line`** instead of copying them.

| Benchmark (6 fields, one quoted) | ns/op | B/op | allocs/op |
|---|---|---|---|
| `Join` (readable) | ~350 | 200 | 5 |
| `JoinFast` | ~255 | 64 | 1 |
| `Split` (readable) | ~340 | 272 | 6 |
| `SplitFast` | ~210 | 136 | 2 |

Correctness is pinned by `FuzzFastMatchesReadable` (a **differential fuzz
test**: the readable function is the oracle). It ran 12M inputs with no
mismatch, and `TestJoinFastAllocatesOnce` pins the allocation promise.

### Trade-off
The readable version is obviously correct at a glance; the fast one has a
size computation that must stay in sync with the writing loop, and its fields
alias the input (fine for strings, which are immutable, but it keeps the whole
line alive while any field is referenced). Choose fast only in a hot path, and
only with a differential test. In real code, use `encoding/csv`.

### Review checklist
- [ ] Fuzz target checks a property. `f.Fuzz(func(t *testing.T, s string) { Split(s) })` only catches panics and **misses bug 1**.
- [ ] Fuzz target ignores `Split`'s error or treats every error as failure (`FuzzSplit` must allow `ErrUnterminated`/`ErrSyntax`).
- [ ] Bug-finding inputs promoted into table tests (with a `// Found by` comment).
- [ ] Fuzz corpus files under `testdata/fuzz/` committed, not deleted.
- [ ] `errors.Is` for sentinel errors; no `err.Error() == "..."`.
- [ ] `t.Fatalf` when a later check depends on an earlier one (e.g. error then fields), `t.Errorf` otherwise.
- [ ] Failure messages name the function and input: `Split(%q) = %q, want %q`.

---

## Exercise 2 — paginate (table tests, boundaries)

### What a good student suite looks like
- `TestPages` table: 0 items, fewer than a page, exact multiple, one over a
  multiple, `perPage == 1`, negative total, `perPage` 0 and negative.
- `TestBounds` table: first / middle / last-partial / last-exact page,
  `total == 0` page 1 → `[0,0)`, page 0, page past the end, invalid sizes,
  errors checked with `errors.Is`.
- Bonus property test `TestBoundsCoverEveryItem`: for `total` 0..40 and
  `perPage` 1..12, walk every page and check the ranges tile `[0,total)` with
  no gap or overlap. **This one test alone catches all three bugs**; point that out to the student as the payoff of
  property tests.

### Readable solution
`Pages` divides and adds one only when there is a remainder; `Bounds`
validates sizes, special-cases the empty list's single page, rejects
`page < 1 || page > Pages(...)`, and clamps `end` with `min`.

### Optimized solution
There is no meaningful speed difference to find here; both versions are a
handful of integer instructions. The common "clever" form is

```go
func Pages(total, perPage int) int {
	if total < 0 || perPage < 1 {
		return 0
	}
	return (total + perPage - 1) / perPage // ceiling division
}

fmt.Println(Pages(0, 10), Pages(21, 10), Pages(20, 10), Pages(10, 0)) // 0 3 2 0
```

It is shorter and branch-free but **overflows** when `total` is near
`math.MaxInt` (the sum wraps negative). The readable version cannot overflow.
A good review point is that "optimizing" this is a correctness risk with no
measurable benefit. Not benchmarked, because there is nothing to measure.

### Trade-off
Keep the readable version. Use the ceiling-division idiom only when inputs are
known to be small (and say so in a comment).

### Review checklist
- [ ] Boundaries tested on **both sides** (exact multiple *and* one over).
- [ ] Error cases use `errors.Is(err, ErrInvalidPage)`; checks the zero values returned with the error.
- [ ] Subtest names are readable (`"last partial page"`, not `"case3"`).
- [ ] No duplicated loops: one table and one `t.Run` per function.
- [ ] `Example` has an `// Output:` comment (otherwise it is compiled but never run).
- [ ] Fix uses `min(start+perPage, total)` (Go 1.21 builtin), not an `if` chain.

---

## Exercise 3 — concat (oracles, AllocsPerRun, benchmarks)

### What a good student suite looks like
- `TestJoinersMatchStringsJoin`: one table of inputs × a slice of joiner
  funcs, comparing each with `strings.Join` (the **oracle**). Must include
  empty strings in the middle, at the end, and all-empty.
- `TestJoinBuilderAllocatesOnce`: `testing.AllocsPerRun` turns the doc
  comment's performance promise into a failing test.
- `BenchmarkJoin` with `b.Run` per joiner and size, inputs built **outside**
  the loop, `for b.Loop()`, `b.ReportAllocs()`.

### Readable vs optimized
Here the three functions *are* the readable-vs-fast spectrum, and the
benchmark is the lesson:

| n elems (8 bytes each) | Plus | Buffer | Builder (fixed) |
|---|---|---|---|
| 10 | ~1.0 µs, 18 allocs | ~290 ns, 3 allocs | ~125 ns, 1 alloc |
| 100 | ~41 µs, 198 allocs | ~2.5 µs, 6 allocs | ~1.0 µs, 1 alloc |
| 1000 | ~3.3 ms, 1998 allocs, 9.6 MB | ~22 µs, 10 allocs | ~7.4 µs, 1 alloc |

`+=` is quadratic (each step copies the whole string so far). `bytes.Buffer`
grows geometrically, and `String()` copies once more at the end.
`strings.Builder` with an exact `Grow` allocates once and `String()` does not
copy. `strings.Join` does exactly what the fixed `JoinBuilder` does; in real
code, call it.

### Trade-off
`+=` is fine for a handful of strings known at compile time (the compiler
handles `a + b + c` in one allocation). In loops, use `strings.Builder`, or
`strings.Join` when you have a slice.

### Review checklist
- [ ] Uses an oracle rather than hand-written expected strings for every case.
- [ ] Builds inputs outside the timed loop; no `b.ResetTimer()` needed with `b.Loop()`.
- [ ] Uses `b.Loop()` (Go 1.24), not `for i := 0; i < b.N; i++`.
- [ ] Doesn't assign to a package-level sink "to defeat the compiler": `b.Loop` keeps the call alive.
- [ ] Allocation promise pinned in a **test**, not only visible in a benchmark nobody reads.
- [ ] Joiner table: `[]struct{name string; join func([]string, string) string}`.

---

## Exercise 4 — reminder (fakes, injected clock)

### What a good student suite looks like
- `fakeClock` with `Now()` and `Advance(d)`; `fakeSender` recording calls
  and failing for configured recipients. Compile-time interface checks
  (`var _ Clock = (*fakeClock)(nil)`).
- A small constructor helper (`newTestScheduler(t)`) with `t.Helper()`.
- Tests: due in the past / **exactly now** / 1ns in the future; clock advance
  then flush; out-of-order adds sent in order, **ties keep insertion order**;
  a failing send is retried on the next flush; multiple failures joined
  (`errors.Is` against each).
- No `time.Sleep`, no `time.Now()` in assertions.

### Readable solution
`Flush` stable-sorts pending by due time, partitions into "send now" and
"keep", keeps failed sends, and joins errors with `errors.Join`. `Pending`
returns a sorted **copy**.

### Optimized solution (reasoning, verified against the reference tests)
Keep `pending` sorted at insert time; then the due reminders are a prefix,
found by binary search:

```go
import "gocourse/solutions/04-testing/exercises/reminder"

type Reminder = reminder.Reminder

type Scheduler struct {
	clock   reminder.Clock
	sender  reminder.Sender
	pending []Reminder // sorted by Due
}

func (s *Scheduler) Add(r Reminder) {
	// Insert after any reminder due at the same time, so ties keep
	// insertion order.
	i, _ := slices.BinarySearchFunc(s.pending, r, func(e, t Reminder) int {
		if e.Due.After(t.Due) {
			return 1
		}
		return -1
	})
	s.pending = slices.Insert(s.pending, i, r)
}

func (s *Scheduler) Flush() (int, error) {
	now := s.clock.Now()
	n, _ := slices.BinarySearchFunc(s.pending, now, func(r Reminder, t time.Time) int {
		if r.Due.After(t) {
			return 1
		}
		return -1
	})
	var (
		failed []Reminder
		errs   []error
		sent   int
	)
	for _, r := range s.pending[:n] {
		if err := s.sender.Send(r.To, r.Body); err != nil {
			errs = append(errs, fmt.Errorf("send reminder to %s: %w", r.To, err))
			failed = append(failed, r)
			continue
		}
		sent++
	}
	// Failed reminders were due before everything still pending, so
	// prepending them keeps the slice sorted.
	s.pending = append(failed, s.pending[n:]...)
	return sent, errors.Join(errs...)
}

type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

type printSender struct{}

func (printSender) Send(to, body string) error {
	if to == "nobody" {
		return errors.New("no such recipient")
	}
	fmt.Println("sent to", to)
	return nil
}

func main() {
	t0 := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	s := &Scheduler{clock: fixedClock(t0), sender: printSender{}}
	s.Add(Reminder{To: "bob", Due: t0.Add(-time.Minute)})
	s.Add(Reminder{To: "later", Due: t0.Add(time.Hour)})
	s.Add(Reminder{To: "nobody", Due: t0.Add(-2 * time.Minute)})
	s.Add(Reminder{To: "ada", Due: t0.Add(-time.Minute)}) // ties keep insertion order
	sent, err := s.Flush()
	fmt.Println(sent, err, len(s.pending)) // 2 send reminder to nobody: no such recipient 2
}
```

`Pending` then becomes a plain `slices.Clone`. This passes the full reference
suite. Flush goes from O(n log n) (re-sorting everything each call) to
O(log n + k) for k due reminders. `Add` becomes O(n) because of the shift,
which suits the expected pattern of frequent flushes and rare adds. Not
benchmarked: the dominant cost in a real system is the `Send` I/O, which
dwarfs either version. For millions of reminders, a `container/heap` is the
standard answer.

### Trade-off
The readable version has one obvious invariant ("sort, then partition"). The
optimized one spreads the invariant ("pending is always sorted") across two
methods, and the tie-breaking comparator is subtle. Choose it only if
profiling shows `Flush` matters.

### Review checklist
- [ ] Fakes, not mocks: tests assert on recorded state, not on scripted call expectations.
- [ ] The boundary "due exactly now" is tested (that is what catches bug 2).
- [ ] Tie ordering tested (catches a non-stable `slices.SortFunc` "fix").
- [ ] Each test builds fresh fakes; no package-level mutable fakes shared between tests.
- [ ] `errors.Is(err, errA) && errors.Is(err, errB)` for joined errors, not `err.Error()` comparison.
- [ ] Fix uses `slices.SortStableFunc` + `time.Time.Compare` (Go 1.20), not a hand-written sort.
- [ ] `SystemClock` is not used in tests except to test `SystemClock` itself.

---

## Exercise 5 — report (golden files)

### What a good student suite looks like
- `var update = flag.Bool("update", false, ...)` and an `assertGolden(t, path, got)`
  helper with `t.Helper()`, that writes on `-update` and otherwise compares
  and prints a readable diff.
- Golden cases: empty bill, basic, **amounts with cents < 10**, **multi-byte
  names**, wide numbers.
- Golden files **read by eye** against `Render`'s doc comment before being
  committed. The most common failure in this exercise is running `-update`
  on the buggy starter and committing the buggy output as "golden". The test
  then passes and catches nothing. The mutation check exposes this at once.
- A golden-independent test (`TestRenderLayoutRules`, `TestMoney`) so a
  wrongly updated golden file cannot hide everything.
- `TestRenderReturnsWriteError` with a failing `io.Writer`.

### Readable solution
Format all cells into strings (header, rows, footer), compute column widths
with `utf8.RuneCountInString`, pad with `fmt`'s `%-*s` / `%*s` (which count
runes), trim trailing spaces, write once. `money` is
`fmt.Sprintf("%d.%02d", cents/100, cents%100)`.

### Optimized solution — `fast.go` (`RenderFast`)
Computes widths arithmetically (digit counts instead of formatting
throw-away strings), then appends everything into one pre-sized `[]byte` with
`strconv.AppendInt` and manual padding, and writes once.

| Benchmark (6 items) | ns/op | B/op | allocs/op |
|---|---|---|---|
| `Render` (readable) | ~8 300 | 4 539 | 113 |
| `RenderFast` | ~650 | 512 | 1 |

That is about 12× faster with 1 allocation instead of 113.
`FuzzRenderFastMatchesRender` (differential, `Render` as oracle) ran 5M
inputs with no mismatch. The existing golden files are a second safety net.

### Trade-off
`RenderFast` is about twice as much code as `Render` (113 lines vs 92 for the whole readable file),
and a layout change (a new column) now touches width maths in three places.
A bill printer is never a hot path: keep the readable one. Show the fast one
to the student as an example of what `fmt` costs, and of how a differential
fuzz test makes such a rewrite safe.

### Review checklist
- [ ] Golden files checked by eye (ask: "what does `testdata/cents.golden` show for a 5-cent item?").
- [ ] `-update` flag declared at package level with `flag.Bool`; no custom `TestMain` needed.
- [ ] Golden write uses `0o644` and handles the `os.WriteFile` error (`t.Fatal`).
- [ ] Missing golden file gives a helpful message ("run with -update").
- [ ] Diff output quotes lines (`%q`) so whitespace differences are visible.
- [ ] `t.Helper()` in `assertGolden`.
- [ ] Unit test for `money` exists; golden files alone make failures hard to localise.
- [ ] Fix counts runes (`utf8.RuneCountInString`), not bytes. Note that this is still wrong for East Asian wide characters or emoji, which take 2 columns; mention it as a known limitation, not a required change.

---

## Project — semver

### What a good student suite looks like
- Black-box test (`package semver_test`) importing the package, so tests use
  only the exported API.
- `TestParse` with the spec's own examples (`1.0.0-x-y-z.--`,
  `1.0.0+21AF26D3---117B344092BD`, build `+001` accepted).
- `TestParseInvalid` with leading zeros in each numeric field, empty
  pre-release / build / identifiers (`1.2.3-`, `1.2.3-a..b`, `1.2.3+`),
  pre-release leading zero (`1.2.3-01`), bad characters, wrong part counts,
  overflow. Every error checked with `errors.Is(err, semver.ErrInvalid)`.
- `TestCompare` table, plus `TestCompareSpecOrder` using the exact precedence
  chain from semver.org §11 (`1.0.0-alpha < 1.0.0-alpha.1 < … < 1.0.0`),
  plus `TestSort` covering stability for build-only differences.
- `FuzzParse` (never panics, errors wrap `ErrInvalid`, accepted input
  round-trips through `String`), `FuzzCompare` (antisymmetry).
- `ExampleCompare`, `ExampleSort`; `BenchmarkParse`, `BenchmarkCompare`.
- Coverage ≥ 90% (`go test -cover`). The reference suite reaches 97.6% of
  `semver.go`; `go tool cover -func` shows the gap is one branch of
  `compareIdent` (87.5%). A nice review exercise is to ask the student to find
  it from the HTML report.

### Readable solution
`Parse` peels off `+build` then `-pre` with `strings.Cut`, validates
identifier lists with `checkIdents`, and splits the core on `.`. `Compare`
compares numbers with `cmp.Compare`, then pre-releases with the §11 rules.
Numeric identifiers are compared **by length first, then lexically**, which
is valid because leading zeros are forbidden, and it never overflows on
huge identifiers. `Sort` uses `slices.SortStableFunc(vs, Compare)`.

### Optimized solution — `fast.go` (`ParseFast`)
Scans the core with `strings.IndexByte` instead of `strings.Split`, and
validates identifiers in place (`checkIdentsFast`) instead of splitting them.
A successful parse allocates nothing.

| Benchmark (`v1.22.333-rc.1.alpha+build.2025`) | ns/op | B/op | allocs/op |
|---|---|---|---|
| `Parse` (readable) | ~210 | 128 | 3 |
| `ParseFast` | ~77 | 0 | 0 |

That is about 2.7× faster with zero allocations. `FuzzParseFastMatchesParse`
(differential) ran 6M inputs with no mismatch, and `TestParseFastDoesNotAllocate`
pins the zero-allocation result. `Compare` (~125 ns, 2 allocs, from
`strings.Split` of the pre-release) could be made allocation-free the same way
by walking both strings with `IndexByte`; this is left as a stretch goal for
the student and not implemented here.

### Trade-off
`Parse` mirrors the grammar and is easy to check against the spec. `ParseFast`
is worth it only if you parse versions in bulk (e.g. a package index with
millions of entries). Error messages from `ParseFast` are slightly less
specific (it reports the whole identifier list, not the bad identifier).

### Review checklist
- [ ] Tests come from the **spec**, not from the implementation (the §11 precedence list is the best single test).
- [ ] `TestParseInvalid` includes input that **panicked** in the starter (`1.2.3-`), as a regression test with a comment.
- [ ] `errors.Is(err, semver.ErrInvalid)`; no string matching of error text.
- [ ] Fuzz properties are real (round-trip, antisymmetry), and `FuzzCompare` skips invalid inputs with `t.Skip()` rather than failing.
- [ ] Numeric comparison fix doesn't use `strconv.Atoi` without handling overflow (`TestCompare/…99999999999999999999…`).
- [ ] Release-vs-pre-release fix handles **both** directions (`a == ""` and `b == ""`).
- [ ] Coverage report checked (`go tool cover -func`), not just the percentage.
- [ ] Examples have `// Output:`.
- [ ] Benchmarks use `b.Loop()` and `b.ReportAllocs()`.

---

## General review checklist (all packages)

- [ ] `gofmt -l` clean; `go vet` clean.
- [ ] Table-driven with `t.Run`; subtest names describe the case.
- [ ] Messages: `Func(input) = got, want want`. Input first, got before want.
- [ ] `t.Helper()` in every helper that calls `t.Error*`/`t.Fatal*`.
- [ ] `t.Fatal*` only when continuing makes no sense; never from a goroutine other than the test's.
- [ ] No `time.Sleep` for synchronisation; time is injected.
- [ ] No test depends on another's side effects (`go test -shuffle=on` passes).
- [ ] `go test -race` passes (relevant if the student added `t.Parallel()`).
- [ ] Tests are readable as documentation of the package's behaviour.

## Common review comments (copy-paste ready)

- "This test passes against the original buggy code too. Re-plant the bug
  and you'll see it doesn't fail. What input would make it fail?"
- "You regenerated the golden files with `-update` before fixing the bug, so
  they record the bug. Open `testdata/cents.golden`: is `0.5` right?"
- "`err.Error() == ...` breaks as soon as someone wraps the error. Use
  `errors.Is`."
- "This fuzz target only checks that nothing panics. What property must
  *always* hold? (Hint: Split undoes Join.)"
- "Use `t.Errorf` here so the remaining cases still run; `t.Fatalf` stops the
  whole subtest."
- "Your `Example` has no `// Output:` comment, so it is compiled but never
  run."
