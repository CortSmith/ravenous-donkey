# Go Resources

A reference page to keep open while you work through the course. Every link here was checked in
October 2026.

> **About the course's Go version.** The course targets **Go 1.24** (`go 1.24.2` in `go.mod`).
> Go releases twice a year and supports only the two newest major versions. As of October 2026,
> those are **Go 1.27** (August 2026) and **Go 1.26** (February 2026). Go 1.24 still works fine,
> but it no longer gets security fixes. Everything you learn here carries forward unchanged.
> See [What's new since Go 1.24](#whats-new-since-go-124) for what's been added since.

---

## 1. Official documentation

| Resource | What it's for |
|---|---|
| [A Tour of Go](https://go.dev/tour/) | Interactive intro to the language |
| [Documentation hub](https://go.dev/doc/) | Index of all official docs and tutorials |
| [How to Write Go Code](https://go.dev/doc/code) | Modules, packages, `go` command basics |
| [The Go Programming Language Specification](https://go.dev/ref/spec) | The definitive language rules. Short and readable |
| [The Go Memory Model](https://go.dev/ref/mem) | What concurrent reads and writes are guaranteed to see. Read it before the concurrency modules |
| [Go FAQ](https://go.dev/doc/faq) | The "why is Go like this?" answers (no exceptions, nil interfaces, ...) |
| [Standard library](https://pkg.go.dev/std) / [pkg.go.dev](https://pkg.go.dev/) | API docs for the stdlib and every public module |
| [Go Playground](https://go.dev/play/) | Run and share snippets in the browser |
| [Release history](https://go.dev/doc/devel/release) | Every release and point release, with dates |
| [Go Toolchains](https://go.dev/doc/toolchain) | How the `go` and `toolchain` lines in `go.mod` select a Go version |
| [Managing dependencies](https://go.dev/doc/modules/managing-dependencies) | `go get`, `go mod tidy`, versions |
| [Organizing a Go module](https://go.dev/doc/modules/layout) | Official guidance on project layout |
| [Go Doc Comments](https://go.dev/doc/comment) | How to write doc comments that render well |
| [Go blog](https://go.dev/blog/) | In-depth articles from the Go team |

Offline docs: `go doc fmt.Printf`, `go doc -all strings`, or `go doc -src sync.Once`.

## 2. Style guides and idioms

Read these in this order. The first two are what your code will be reviewed against.

1. **[Effective Go](https://go.dev/doc/effective_go)**: the classic guide to idiomatic Go. It was
   written in 2009, so it doesn't cover modules or generics, but its core idioms (naming, errors,
   interfaces, concurrency) still hold.
2. **[Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)**: a checklist of common
   review feedback, such as receiver names, error strings, `context` placement and
   "accept interfaces, return structs". Its companion is
   [Go Test Comments](https://go.dev/wiki/TestComments).
3. **[Google Go Style Guide](https://google.github.io/styleguide/go/)**: the most thorough modern
   guide. It has three parts:
   [Style Guide](https://google.github.io/styleguide/go/),
   [Style Decisions](https://google.github.io/styleguide/go/decisions) and
   [Best Practices](https://google.github.io/styleguide/go/best-practices).
4. **[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)**: practical,
   example-heavy guidance (guidelines, performance, patterns). Some of it is Uber-specific
   preference, so where it differs from the guides above, follow those.
5. **[Go Proverbs](https://go-proverbs.github.io/)**: Rob Pike's one-liners, e.g. "Don't communicate
   by sharing memory; share memory by communicating", "The bigger the interface, the weaker the
   abstraction", "Errors are values", "Clear is better than clever".

### Common mistakes

- **[100 Go Mistakes and How to Avoid Them](https://100go.co/)** by Teiva Harsanyi. The website
  summarises every mistake for free. The book is published by
  [Manning](https://www.manning.com/books/100-go-mistakes-and-how-to-avoid-them).
- [Go Wiki: Common Mistakes](https://go.dev/wiki/CommonMistakes)

### Must-read Go blog articles

| Topic | Article |
|---|---|
| Errors | [Errors are values](https://go.dev/blog/errors-are-values) · [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors) |
| Concurrency | [Pipelines and cancellation](https://go.dev/blog/pipelines) · [Context](https://go.dev/blog/context) · [Concurrency patterns talk (2012)](https://go.dev/talks/2012/concurrency.slide) |
| Maps | [Go maps in action](https://go.dev/blog/maps) · [Swiss table maps (1.24)](https://go.dev/blog/swisstable) |
| Generics | [An Introduction to Generics](https://go.dev/blog/intro-generics) · [When to Use Generics](https://go.dev/blog/when-generics) · [Generic interfaces](https://go.dev/blog/generic-interfaces) · [Tutorial](https://go.dev/doc/tutorial/generics) |
| Iterators | [Range over function types](https://go.dev/blog/range-functions) |
| Loops | [Fixing for loops in Go 1.22](https://go.dev/blog/loopvar-preview) |
| HTTP | [Routing enhancements (1.22)](https://go.dev/blog/routing-enhancements) |
| Logging | [Structured logging with slog](https://go.dev/blog/slog) |
| Testing | [More predictable benchmarking with `testing.B.Loop`](https://go.dev/blog/testing-b-loop) · [Fuzzing tutorial](https://go.dev/doc/tutorial/fuzz) · [Data race detector](https://go.dev/doc/articles/race_detector) |
| Files | [Traversal-resistant file APIs (`os.Root`)](https://go.dev/blog/osroot) |
| Performance | [Profiling Go programs](https://go.dev/blog/pprof) · [Diagnostics](https://go.dev/doc/diagnostics) · [GC guide](https://go.dev/doc/gc-guide) · [PGO](https://go.dev/doc/pgo) |
| Security | [Security best practices](https://go.dev/doc/security/best-practices) |

## 3. Tooling

### Built into the `go` command

```sh
go fmt ./...            # format (runs gofmt -l -w on packages)
gofmt -s -l .           # list unformatted files; -s also simplifies code
go vet ./...            # catch suspicious code (printf args, copied locks, bad struct tags, ...)
go test ./...           # run tests
go test -race ./...     # run tests with the data race detector
go test -cover ./...    # coverage summary
go test -coverprofile=c.out ./... && go tool cover -html=c.out
go doc <pkg>[.<sym>]    # documentation in the terminal
go mod tidy             # sync go.mod/go.sum with imports
```

### External tools (optional, not needed for the course)

The course uses only the standard library, but these are what Go teams use day to day:

| Tool | What it does | Install |
|---|---|---|
| [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) | `gofmt` plus adding and removing imports | `go install golang.org/x/tools/cmd/goimports@latest` |
| [gopls](https://go.dev/gopls/) | The official language server, used by VS Code (Go extension), Neovim, Emacs and other LSP editors | `go install golang.org/x/tools/gopls@latest` (your editor usually installs it for you) |
| [staticcheck](https://staticcheck.dev/) | High-signal static analysis: bugs, simplifications, deprecated APIs | `go install honnef.co/go/tools/cmd/staticcheck@latest`, then `staticcheck ./...` |
| [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) | Reports known vulnerabilities that your code actually calls ([tutorial](https://go.dev/doc/tutorial/govulncheck)) | `go install golang.org/x/vuln/cmd/govulncheck@latest`, then `govulncheck ./...` |
| [golangci-lint](https://golangci-lint.run/) | Runs many linters at once (staticcheck, errcheck, revive, gosec, ...) | Binary (recommended): `curl -sSfL https://golangci-lint.run/install.sh \| sh -s -- -b $(go env GOPATH)/bin v2.14.0`, or `brew install golangci-lint`. Then `golangci-lint run ./...` |
| [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) | Statistically compares benchmark runs | `go install golang.org/x/perf/cmd/benchstat@latest` |

> **Heads-up for Go 1.24 users:** the latest releases of these tools require Go ≥ 1.26 to *build*.
> With the default `GOTOOLCHAIN=auto`, `go install ...@latest` quietly downloads a newer Go
> toolchain to build the tool, and that works. The tools still analyze your Go 1.24 code fine. If
> you've set `GOTOOLCHAIN=local`, the install fails with "requires go >= 1.26.0". In that case,
> either unset it or pin an older tool version.

A minimal **golangci-lint v2** config, `.golangci.yml` (v1 configs need
`golangci-lint migrate`):

```yaml
version: "2"
linters:
  default: standard     # standard | all | none | fast
  enable:
    - revive
    - gosec
formatters:
  enable:
    - gofmt
    - goimports
run:
  timeout: 5m
```

### Profiling and performance

```sh
# Benchmarks: -run='^$' skips tests; -benchmem shows B/op and allocs/op
go test -run='^$' -bench=. -benchmem ./path/to/pkg
go test -run='^$' -bench=WordCount -benchtime=2s -count=10 ./path/to/pkg

# Compare before and after with benchstat (use at least 10 runs)
go test -run='^$' -bench=. -count=10 ./pkg > old.txt
#   ...make your change...
go test -run='^$' -bench=. -count=10 ./pkg > new.txt
benchstat old.txt new.txt

# CPU and memory profiles from benchmarks (one package at a time)
go test -run='^$' -bench=. -cpuprofile=cpu.out -memprofile=mem.out ./pkg
go tool pprof -top cpu.out                 # hottest functions
go tool pprof -list='WordCount' cpu.out    # annotated source
go tool pprof -http=:8080 cpu.out          # interactive web UI (flame graph, graph, source)
go tool pprof -sample_index=alloc_space mem.out

# Escape analysis: why does this allocate?
go build -gcflags=-m ./pkg                 # "moved to heap: x", "... escapes to heap", inlining
go build -gcflags='-m -m' ./pkg            # verbose reasons

# Execution trace (goroutine scheduling, GC, blocking)
go test -trace=trace.out ./pkg && go tool trace trace.out
```

**Profile-guided optimization (PGO):** save a production CPU profile as `default.pgo` in your main
package's directory. `go build` uses it automatically (since 1.21), which usually gives a 2–7%
speedup. See [the PGO guide](https://go.dev/doc/pgo).

Live servers: `import _ "net/http/pprof"` exposes `/debug/pprof/` (e.g.
`go tool pprof 'http://localhost:6060/debug/pprof/profile?seconds=30'`). Never expose it publicly.

## 4. Modern Go features you'll use (by version)

All of these are available in Go 1.24.

| Version | Feature |
|---|---|
| 1.18 | Generics (`[T any]`, constraints, `comparable`), fuzzing (`go test -fuzz`), workspaces |
| 1.20 | `errors.Join` and multiple `%w`, `context.WithCancelCause`, `strings.CutPrefix/CutSuffix`, `time.DateOnly` |
| 1.21 | `min`/`max`/`clear` builtins; `slices`, `maps`, `cmp`, `log/slog` packages; `sync.OnceFunc/OnceValue`; `context.AfterFunc/WithoutCancel`; PGO |
| 1.22 | **Each loop iteration gets a fresh variable** (if `go.mod` says ≥ 1.22); `for i := range 10`; `http.ServeMux` patterns like `"GET /items/{id}"` with `r.PathValue("id")`; `math/rand/v2`; `cmp.Or`; `slices.Concat` |
| 1.23 | Range-over-func **iterators** and the `iter` package; `slices.Collect/Sorted/Chunk`, `maps.Keys/Values/All` (these return iterators); `unique` |
| 1.24 | Generic type aliases; `for b.Loop()` benchmarks; `t.Context()`, `t.Chdir()`; `os.Root` (sandboxed file access); `strings.Lines/SplitSeq/FieldsSeq`; `json:",omitzero"`; `weak`, `runtime.AddCleanup`; `tool` directive in `go.mod` |

Note: `testing/synctest` exists in 1.24 only as an experiment (`GOEXPERIMENT=synctest`). The
stable API is from 1.25 and is different, so the course doesn't use it.

## 5. What's new since Go 1.24

You won't need these in the course, but you'll see them in newer codebases.

**Go 1.25** (Aug 2025, [notes](https://go.dev/doc/go1.25), [blog](https://go.dev/blog/go1.25))
- `wg.Go(func() { ... })` on `sync.WaitGroup`, which replaces the `Add(1)` / `defer Done()` dance.
- `testing/synctest` is stable: `synctest.Test(t, func(t *testing.T) { ... })` runs concurrent code
  on a fake clock ([blog](https://go.dev/blog/synctest)).
- GOMAXPROCS respects container CPU limits.
- `encoding/json/v2` is available as an experiment ([blog](https://go.dev/blog/jsonv2-exp)).
- `net/http.CrossOriginProtection` (CSRF), plus new `go vet` checks for misplaced `wg.Add` and for
  building `host:port` strings with `Sprintf`.

**Go 1.26** (Feb 2026, [notes](https://go.dev/doc/go1.26), [blog](https://go.dev/blog/go1.26))
- `new` accepts an expression: `Age: new(yearsSince(born))`, which is handy for optional pointer fields.
- The **Green Tea garbage collector** is on by default and cuts GC overhead by 10–40%
  ([blog](https://go.dev/blog/greenteagc)).
- `errors.AsType[*MyErr](err)` is the generic, type-safe form of `errors.As`.
- `go fix` was rewritten around "modernizers" that update code to current idioms
  ([blog](https://go.dev/blog/gofix)).
- `slog.NewMultiHandler`, a faster `io.ReadAll`, and pprof's web UI opening on a flame graph.

**Go 1.27** (Aug 2026, [notes](https://go.dev/doc/go1.27), [blog](https://go.dev/blog/go1.27))
- **Generic methods:** methods can now declare their own type parameters (interface methods
  can't).
- `encoding/json` now runs on the v2 implementation, and `encoding/json/v2` / `jsontext` are
  stable. Old code keeps working.
- A new `uuid` package, `strings.CutLast`, and `synctest.Sleep`.
- The goroutine-leak profile (`/debug/pprof/goroutineleak`) is stable.

To try a newer Go without changing your system install, run
`go install golang.org/dl/go1.27.2@latest && go1.27.2 download`.

## 6. Books and learning sites

- *The Go Programming Language*, Donovan & Kernighan ([gopl.io](https://www.gopl.io/)). Still the
  best deep treatment of the core language (pre-generics).
- *100 Go Mistakes and How to Avoid Them*, Teiva Harsanyi ([100go.co](https://100go.co/)).
- *Learning Go* (2nd ed., covers generics), Jon Bodner, O'Reilly.
- *Concurrency in Go*, Katherine Cox-Buday, O'Reilly.
- *Let's Go* / *Let's Go Further*, Alex Edwards ([lets-go.alexedwards.net](https://lets-go.alexedwards.net/)).
  Building web apps and APIs with the standard library.
- [Learn Go with Tests](https://quii.gitbook.io/learn-go-with-tests): learn by doing TDD.
- [Go by Example](https://gobyexample.com/): short annotated programs for each feature.
