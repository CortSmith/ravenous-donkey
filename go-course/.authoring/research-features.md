# Go feature research notes (for module authors)

Researched 2026-10-11. Sources: go.dev/doc/go1.NN release notes, go.dev/blog, pkg.go.dev, and
`$GOROOT/api/go1.NN.txt` from the local go1.24.2 (that's the stdlib's own record of which release
added each symbol, so treat it as authoritative). Every code snippet marked [compiled] was built,
vetted and run with go1.24.2 under a `go 1.24` go.mod (see scratchpad/verify/).

## 0. Headline facts and corrections

- **Release status (Oct 2026):** go1.24 (2025-02-11), go1.25 (2025-08-12), go1.26 (2026-02-10),
  **go1.27 (2026-08-19)**. Latest point releases are go1.26.9 and go1.27.2 (2026-10-08). Only the
  two newest majors get support, so **1.24 and 1.25 are both out of support**. go1.24.13
  (2026-02-04) was the last 1.24 point release. The course pins 1.24 on purpose. Say so in prose,
  and don't present 1.24 as "current".
- **`maps.Keys`/`maps.Values` are Go 1.23, not 1.21.** They return `iter.Seq`, not slices. The 1.21
  `maps` package only has Clone, Copy, DeleteFunc, Equal, EqualFunc. (Some summaries wrongly list
  Keys/Values in 1.21: they were in x/exp/maps and were dropped before 1.21 shipped.) To get a
  sorted key slice, use `slices.Sorted(maps.Keys(m))` (1.23).
- **`cmp.Or` is 1.22.** `cmp.Compare`, `cmp.Less` and `cmp.Ordered` are 1.21.
- **`math/rand/v2` is 1.22.**
- **`unique` is 1.23.** `weak` and `runtime.AddCleanup` are 1.24.
- **`testing/synctest` in 1.24 is experimental.** It only exists with `GOEXPERIMENT=synctest` and its
  API is `synctest.Run(func())` + `synctest.Wait()`. It went GA in **1.25** with a *different* entry
  point, `synctest.Test(t, func(t *testing.T))`. Current docs (1.27) list Test, Wait and Sleep (1.27),
  and `Run` no longer exists. **The course must not depend on it** (it needs a GOEXPERIMENT, and the
  API changed).
- **`time.DateTime` is `"2006-01-02 15:04:05"`** (with a space, not a T). It was added in 1.20.
- **Tooling trap:** the latest staticcheck (v0.8.1), govulncheck (v1.9.0), gopls (v0.23.0),
  golangci-lint (v2.14.0) and x/perf (benchstat) all declare `go 1.26.0` in go.mod. With go1.24.2
  and the default `GOTOOLCHAIN=auto`, `go install pkg@latest` **auto-downloads go1.26.x and
  succeeds** (verified with govulncheck). With `GOTOOLCHAIN=local` it fails with
  "requires go >= 1.26.0". Don't use `go get -tool` in a 1.24 module for these tools, because it
  would bump the module's `go` line to 1.26. Last versions that build with ≤1.24: staticcheck
  v0.6.1 (2025.1.1; go 1.23), gopls v0.20.0 (go 1.24.2). govulncheck v1.4.0+ needs 1.25.
- `B.Loop` in 1.24 prevents inlining of the loop body. Go 1.26 fixed that ("B.Loop no longer
  prevents inlining"). This doesn't matter much for the course, but explains odd results in a few
  micro-benchmarks.

## 1. Features by version (1.18–1.24)

| Feature | Ver | One-liner |
|---|---|---|
| Generics (type params, constraints, `any`, `comparable`) | 1.18 | Parametric funcs/types |
| Fuzzing (`go test -fuzz`, `testing.F`) | 1.18 | Native coverage-guided fuzzing |
| Workspaces (`go work`) | 1.18 | Multi-module dev |
| `errors.Join`, multiple `%w` in `fmt.Errorf` | 1.20 | Multi-error wrapping; Is/As walk trees |
| `context.WithCancelCause`, `context.Cause` | 1.20 | Cancel with an error reason |
| Slice→array conversion `[4]byte(s)` | 1.20 | Panics if len(s) < 4 |
| `strings/bytes.CutPrefix/CutSuffix` | 1.20 | Trim + report |
| `time.DateTime/DateOnly/TimeOnly` | 1.20 | Layout consts |
| `http.ResponseController` | 1.20 | Per-request deadlines/flush |
| `go build -cover` | 1.20 | Coverage for binaries |
| `min`, `max`, `clear` builtins | 1.21 | `clear(map)` deletes all; `clear(slice)` zeroes elements (len unchanged) |
| `slices`, `maps`, `cmp` packages | 1.21 | Generic helpers (maps: Clone/Copy/DeleteFunc/Equal/EqualFunc only) |
| `log/slog` | 1.21 | Structured logging (Text/JSON handlers) |
| `sync.OnceFunc/OnceValue/OnceValues` | 1.21 | Lazy init helpers |
| `context.AfterFunc`, `WithoutCancel`, `WithDeadlineCause`, `WithTimeoutCause` | 1.21 | |
| `errors.ErrUnsupported` | 1.21 | |
| PGO GA (`default.pgo`, `-pgo=auto` default) | 1.21 | (preview 1.20) |
| `toolchain` directive / `GOTOOLCHAIN` | 1.21 | Auto toolchain download |
| Per-iteration loop variables | 1.22 | **Gated on go.mod `go` ≥ 1.22** |
| `for i := range n` (range over int) | 1.22 | |
| ServeMux method + wildcard patterns, `Request.PathValue` | 1.22 | `GODEBUG=httpmuxgo121=1` reverts |
| `math/rand/v2`, `slices.Concat`, `cmp.Or`, `sql.Null[T]`, `go/version` | 1.22 | |
| Range-over-func iterators; `iter` package (`Seq`, `Seq2`, `Pull`, `Pull2`) | 1.23 | (preview 1.22 via GOEXPERIMENT=rangefunc) |
| `slices.All/Values/Backward/Collect/AppendSeq/Sorted/SortedFunc/SortedStableFunc/Chunk/Repeat` | 1.23 | |
| `maps.All/Keys/Values/Insert/Collect` | 1.23 | Iterator-based |
| `unique` package (`Make[T]`, `Handle[T]`) | 1.23 | Interning |
| Timer/Ticker: unbuffered chans, unstopped timers GC-able | 1.23 | Gated on go.mod ≥ 1.23 |
| `structs.HostLayout`, `go telemetry`, vet `stdversion` | 1.23 | |
| Generic type aliases `type Set[T comparable] = map[T]struct{}` | 1.24 | Full support (preview 1.23) |
| `tool` directive in go.mod, `go get -tool`, `go tool <name>` | 1.24 | Replaces tools.go |
| `testing.B.Loop`, `T/B/F.Context()`, `T/B/F.Chdir()` | 1.24 | |
| `os.Root` / `os.OpenRoot` | 1.24 | Dir-confined file access |
| `strings`/`bytes` `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq`, `FieldsFuncSeq` | 1.24 | Iterator variants |
| `encoding/json` `omitzero` tag | 1.24 | Omit zero value, or when `IsZero() bool` returns true |
| `weak` package (`weak.Make`, `Pointer[T].Value`) | 1.24 | Weak pointers |
| `runtime.AddCleanup` | 1.24 | Better finalizers |
| `crypto/rand.Text`, `crypto/sha3`, `hkdf`, `pbkdf2`, `mlkem` | 1.24 | |
| `testing/synctest` | 1.24 **experimental** | GOEXPERIMENT=synctest; GA 1.25 with different API |
| Swiss-table maps runtime | 1.24 | Faster maps, transparent |
| vet `tests` analyzer; printf flags `Printf(s)` with non-const s | 1.24 | |

**Not in 1.24 (don't use):** `sync.WaitGroup.Go` (1.25), `synctest.Test` (1.25),
`encoding/json/v2` (experiment 1.25, GA 1.27), `errors.AsType` (1.26), `new(expr)` (1.26),
generic methods (1.27), `strings.CutLast`/`bytes.CutLast` (1.27), `uuid` package (1.27),
`slog.NewMultiHandler` (1.26), `slog.GroupAttrs` (1.25), `reflect.TypeAssert` (1.25),
`T.Attr`/`T.Output` (1.25), `T.ArtifactDir` (1.26), `net/http.CrossOriginProtection` (1.25),
`os.Root` methods `ReadFile`/`WriteFile`/`MkdirAll`/`RemoveAll`/`Rename`/`Chmod`/etc. (1.25; in 1.24
Root has only Open, Create, OpenFile, OpenRoot, Mkdir, Remove, Stat, Lstat, FS, Close, Name, which
was verified with `go doc os.Root` against go1.24.2. There's also a package-level
`os.OpenInRoot(dir, name)`.)

## 2. Code examples ([compiled] unless noted)

### Generics (1.18)
```go
type Number interface{ ~int | ~int64 | ~float64 }

func Sum[T Number](xs []T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }
func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	v := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return v, true
}
```
Methods can't have their own type params in 1.24 (generic methods only arrived in 1.27).

### slices / maps / cmp / builtins (1.21–1.23)
```go
xs := []int{3, 1, 2}
slices.Sort(xs)                       // [1 2 3]
i, found := slices.BinarySearch(xs, 2) // 1 true
slices.Contains(xs, 3); slices.Index(xs, 3); slices.Max(xs)
min(3, 1, 2); max(2.5, 1.0)           // builtins, any ordered type
slices.SortFunc(ps, func(a, b P) int { // multi-key sort
	return cmp.Or(cmp.Compare(b.Age, a.Age), strings.Compare(a.Name, b.Name)) // cmp.Or: 1.22
})
m2 := maps.Clone(m)
clear(m)                               // map now empty
clear(s)                               // slice elements zeroed, len unchanged
keys := slices.Sorted(maps.Keys(m2))   // 1.23
slices.Concat([]int{1}, []int{2, 3})   // 1.22
for c := range slices.Chunk(xs, 2) {}  // 1.23
```

### Loop variables + range over int (1.22)
```go
var fns []func() int
for i := range 3 { fns = append(fns, func() int { return i }) } // 0 1 2
```
Verified: a 3-clause `for i := 0; i < 3; i++` closure loop prints `3 3 3` under `go 1.21` in
go.mod and `0 1 2` under `go 1.22`. The semantics follow the **module's go line**, not the
toolchain version.

### Iterators (1.23)
```go
func Countdown(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := n; i > 0; i-- {
			if !yield(i) { // MUST stop when yield returns false (break), or it panics at runtime
				return
			}
		}
	}
}
func (s *Stack[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, v := range s.items {
			if !yield(i, v) { return }
		}
	}
}
for n := range Countdown(10) { if n < 8 { break } }
slices.Collect(Countdown(3)) // [3 2 1]
```
Pull-style: `next, stop := iter.Pull(seq); defer stop(); v, ok := next()`.
Naming convention from the stdlib: `All()` returns the full sequence; `Keys`/`Values`/`Backward`
are variants; a `...Seq` suffix marks an iterator version of an existing slice-returning func.

### strings iterators (1.24)
```go
for line := range strings.Lines("a\nb\nc") {} // "a\n", "b\n", "c" (keeps terminators!)
for f := range strings.SplitSeq("a,b,c", ",") {}
for f := range strings.FieldsSeq("  x  y ") {}
```

### net/http ServeMux (1.22)
```go
mux := http.NewServeMux()
mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "item %s", r.PathValue("id"))
})
mux.HandleFunc("GET /files/{path...}", h) // rest of path, e.g. "a/b.txt"
mux.HandleFunc("GET /{$}", home)          // exactly "/" only
```
Verified behaviour: POST to a GET-only route gives **405** with `Allow: GET, HEAD`. "GET" also
matches HEAD. An unmatched path gives 404. If two patterns conflict and neither is more specific,
registration panics.

### log/slog (1.21)
```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
logger.Info("user logged in", "user", "ann", "attempts", 3)
logger.With("req_id", "abc").Debug("details", slog.Int("n", 1))
slog.New(slog.NewJSONHandler(os.Stdout, nil)).Warn("disk", slog.Group("usage", "pct", 91))
// {"time":...,"level":"WARN","msg":"disk","usage":{"pct":91}}
```
`go vet` (1.22+) flags mismatched key/value pairs in slog calls.

### errors (1.20)
```go
joined := errors.Join(errA, errB)                 // Error(): "a\nb"
errors.Is(joined, errB)                            // true
w := fmt.Errorf("both: %w and %w", errA, errB)     // multiple %w OK since 1.20
```

### context (1.20/1.21)
```go
ctx, cancel := context.WithCancelCause(parent)
cancel(errors.New("shutting down"))
ctx.Err()            // context.Canceled
context.Cause(ctx)   // "shutting down"

stop := context.AfterFunc(ctx, func() { /* runs in own goroutine after ctx done */ })
_ = stop             // stop() bool deregisters
detached := context.WithoutCancel(ctx) // keeps values, never canceled (e.g., async audit log)
ctx3, c3 := context.WithTimeoutCause(parent, time.Second, errors.New("too slow"))
// after expiry: ctx3.Err()==DeadlineExceeded, Cause(ctx3)=="too slow"
```

### sync.Once helpers (1.21)
```go
load := sync.OnceValue(func() int { return expensive() })  // load() memoized, concurrency-safe
cfg := sync.OnceValues(func() (string, error) { return os.Getwd() })
initOnce := sync.OnceFunc(func() { ... })                   // if f panics, every call re-panics
```

### encoding/json omitzero (1.24)
```go
type Person struct {
	Name string    `json:"name"`
	Born time.Time `json:"born,omitzero"` // omitempty never omits a struct; omitzero does (uses IsZero())
}
// json.Marshal(Person{Name: "Ann"}) -> {"name":"Ann"}
```

### os.Root (1.24)
```go
root, err := os.OpenRoot(dir)
defer root.Close()
f, err := root.Create("ok.txt")
_, err = root.Open("../../etc/passwd") // error: "openat ../../etc/passwd: path escapes from parent"
```
It also blocks symlinks that point outside the root. Blog: go.dev/blog/osroot.

### testing (1.24)
```go
func BenchmarkJoin(b *testing.B) {
	parts := []string{"a", "b", "c"} // setup excluded from timing automatically
	for b.Loop() {
		_ = strings.Join(parts, ",")   // results are kept alive (no dead-code elimination)
	}
}
func TestX(t *testing.T) {
	ctx := t.Context()     // canceled just before Cleanup functions run
	t.Chdir(t.TempDir())   // restored after the test; can't be used with t.Parallel
}
```
Rules for `b.Loop`: exactly one `b.Loop` loop per benchmark, and don't mix it with a `b.N` loop.
The function body runs once per `-count`, not repeatedly like b.N. If you need per-iteration setup,
you still have to use `b.StopTimer`/`b.StartTimer`. `b.ReportAllocs()` still works, or pass
`-benchmem`.

### testing/synctest: 1.24 EXPERIMENTAL (don't use in the course)
```go
//go:build goexperiment.synctest
synctest.Run(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	time.Sleep(4 * time.Second) // fake clock; instant
	synctest.Wait()
	// ctx.Err()==nil; sleep 1s more -> DeadlineExceeded
})
```
[compiled with `GOEXPERIMENT=synctest go test`]. The 1.25+ form is
`synctest.Test(t, func(t *testing.T) {...})`.

### generic type alias (1.24)
```go
type Set[T comparable] = map[T]struct{}
```

### unique (1.23), weak + AddCleanup (1.24)
```go
h1 := unique.Make("hello"); h2 := unique.Make(strings.Clone("hello"))
h1 == h2 // true: pointer-cheap comparison; h1.Value() == "hello"

wp := weak.Make(bl)            // weak.Pointer[Blob]; wp.Value() returns nil once collected
runtime.AddCleanup(bl, func(name string) { fmt.Println("cleanup", name) }, "blob")
// signature: AddCleanup[T, S any](ptr *T, cleanup func(S), arg S) runtime.Cleanup  (Cleanup.Stop())
// arg/cleanup must NOT reference ptr (or the object is never collected). Not guaranteed to run.
```
These are advanced, low-level APIs. At most, mention them in a memory/GC module.

## 3. After 1.24: worth knowing about, not used in the course

**Go 1.25 (2025-08-12):** no language changes ("core types" removed from the spec).
`sync.WaitGroup.Go(func())`. `testing/synctest` GA (`Test`, `Wait`). `encoding/json/v2` and
`encoding/json/jsontext` behind `GOEXPERIMENT=jsonv2`. Container-aware GOMAXPROCS (cgroup limits,
updated periodically). Green Tea GC experiment (`GOEXPERIMENT=greenteagc`).
`runtime/trace.FlightRecorder`. `net/http.CrossOriginProtection`. New vet checks `waitgroup`
(misplaced wg.Add) and `hostport`. `go.mod` `ignore` directive. `go doc -http`. `T.Attr`,
`T.Output`. `slog.GroupAttrs`. `reflect.TypeAssert`. Also fixed: a 1.21 compiler bug that delayed
nil checks (code that used `f.Name()` before checking `err` now panics correctly).

**Go 1.26 (2026-02-10):** `new(expr)`, e.g. `Age: new(yearsSince(born))` (release-notes example; `new(42)` with an untyped constant is UNVERIFIED).
Self-referential generic constraints (`type Adder[A Adder[A]] interface{...}`). **Green Tea GC is
on by default** (10–40% less GC overhead). Rewritten `go fix` with "modernizers" and
`//go:fix inline`. `errors.AsType[T](err)`. `slog.NewMultiHandler`. `bytes.Buffer.Peek`. io.ReadAll
is about 2x faster. pprof `-http` opens on the flame graph by default. Goroutine leak profile
(experiment). `crypto/hpke`. `simd/archsimd` (experiment, amd64). cgo calls are ~30% cheaper.
`T.ArtifactDir`.

**Go 1.27 (2026-08-19):** **Generic methods** (a method can declare its own type params; interface
methods can't). Struct-literal keys can be field selectors (promoted fields). Generalized function
type inference. **`encoding/json` is now backed by v2, and `encoding/json/v2` + `jsontext` are GA.**
v1 behaviour is preserved, but error text may differ. New `uuid` package. `goroutineleak` pprof
profile is GA (`/debug/pprof/goroutineleak`). `strings.CutLast`/`bytes.CutLast`. `synctest.Sleep`.
`httptest.NewTestServer` (in-memory, for synctest). `crypto/mldsa`. Portable `simd` experiment. The
`asynctimerchan` GODEBUG is removed (timer chans are always synchronous). `go test` runs vet
`stdversion` by default. `go doc pkg@version`.

## 4. Profiling / performance commands (verified syntax)

```sh
go test -run='^$' -bench=. -benchmem ./pkg            # skip tests, run all benchmarks, show B/op allocs/op
go test -run='^$' -bench=Join -benchtime=2s -count=10 ./pkg
go test -run='^$' -bench=. -cpuprofile=cpu.out -memprofile=mem.out ./pkg   # one package at a time
go tool pprof -top cpu.out                            # text; also -list=FuncRegex, -web
go tool pprof -http=:8080 cpu.out                     # web UI (flame graph default since 1.26)
go tool pprof -sample_index=alloc_space mem.out       # allocations vs inuse_space
go build -gcflags=-m ./pkg                            # escape analysis + inlining decisions; -m -m for reasons
go build -gcflags='all=-m' .                          # include deps
go test -race ./...
go test -coverprofile=c.out ./... && go tool cover -html=c.out
go test -fuzz=FuzzX -fuzztime=30s ./pkg               # one package; one fuzz target
# benchstat
go install golang.org/x/perf/cmd/benchstat@latest
go test -run='^$' -bench=. -count=10 > old.txt   # ≥10 runs, ideally 20 (benchstat docs)
go test -run='^$' -bench=. -count=10 > new.txt
benchstat old.txt new.txt
# runtime tracing
go test -trace=trace.out ./pkg && go tool trace trace.out
# PGO (GA 1.21): put a CPU profile at main pkg dir as default.pgo; -pgo=auto is default
curl -o default.pgo 'http://localhost:6060/debug/pprof/profile?seconds=30'   # needs import _ "net/http/pprof"
go tool pprof -proto a.pprof b.pprof > merged.pprof   # merge profiles
go build -pgo=off .                                   # disable
```
Example `-gcflags=-m` output: `./main.go:50:9: func literal escapes to heap`,
`moved to heap: x`. Docs: go.dev/doc/diagnostics, go.dev/doc/gc-guide, go.dev/doc/pgo,
go.dev/blog/pprof.

## 5. Linters and tooling

| Tool | Install | Run |
|---|---|---|
| go vet | built in | `go vet ./...` |
| gofmt | built in | `gofmt -l -w .` (or `go fmt ./...`); `gofmt -s` simplifies |
| goimports | `go install golang.org/x/tools/cmd/goimports@latest` | `goimports -w .` |
| staticcheck | `go install honnef.co/go/tools/cmd/staticcheck@latest` | `staticcheck ./...` |
| govulncheck | `go install golang.org/x/vuln/cmd/govulncheck@latest` | `govulncheck ./...` |
| gopls | `go install golang.org/x/tools/gopls@latest` (editors usually install it for you) | LSP |
| golangci-lint v2 | **binary is recommended**: `curl -sSfL https://golangci-lint.run/install.sh \| sh -s -- -b $(go env GOPATH)/bin v2.14.0`; `brew install golangci-lint`; `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0` works but the docs **discourage** it | `golangci-lint run ./...`, `golangci-lint fmt` |
| benchstat | `go install golang.org/x/perf/cmd/benchstat@latest` | see above |

All `@latest` versions need go ≥1.26. GOTOOLCHAIN=auto handles this (see section 0).

Minimal golangci-lint v2 config, `.golangci.yml` (from the official docs):
```yaml
version: "2"
linters:
  default: standard        # standard | all | none | fast
  enable:
    - revive
    - gosec
  # settings: {...}  exclusions: {...}
formatters:
  enable:
    - gofmt
    - goimports
run:
  timeout: 5m
```
In v2, formatters are a separate section from linters. Use `golangci-lint migrate` to convert a v1
config. The `standard` set is errcheck, govet, ineffassign, staticcheck and unused (UNVERIFIED that
this exact list is current). gosimple and stylecheck were merged into `staticcheck` in v2.

## 6. Style and reference URLs (all returned HTTP 200 on 2026-10-11)

- Effective Go: https://go.dev/doc/effective_go (written in 2009 and largely unchanged, so it
  doesn't cover generics or modules)
- Go Code Review Comments: https://go.dev/wiki/CodeReviewComments ; Test comments:
  https://go.dev/wiki/TestComments
- Google Go Style Guide: https://google.github.io/styleguide/go/ (+ /decisions, /best-practices)
- Uber Go Style Guide: https://github.com/uber-go/guide/blob/master/style.md
- Go Proverbs: https://go-proverbs.github.io/
- 100 Go Mistakes (Teiva Harsanyi): https://100go.co/ ; book:
  https://www.manning.com/books/100-go-mistakes-and-how-to-avoid-them
- Go wiki Common Mistakes: https://go.dev/wiki/CommonMistakes
- FAQ https://go.dev/doc/faq · Spec https://go.dev/ref/spec · Memory model https://go.dev/ref/mem
- Doc comments: https://go.dev/doc/comment · Module layout: https://go.dev/doc/modules/layout
- Blogs: range-functions, loopvar-preview, routing-enhancements, slog, testing-b-loop, osroot,
  cleanups-and-weak, unique, intro-generics, when-generics, swisstable, synctest, jsonv2-exp,
  greenteagc, pgo, alias-names, go1.13-errors, errors-are-values, pipelines, context, pprof. All are
  at https://go.dev/blog/<slug>.
- O'Reilly book pages return 403 to scripts, so their links are unverified (left out of RESOURCES.md).
