# Module briefs

Student profile: has written a tiny Go program (see Module 00 notes); comfortable programmer, new-ish to Go idioms. Wants in-depth lessons, code review, readable vs. optimized solutions, projects < 1 hour.

## 00-setup — "Setup & First Program" (replaced 00-first-review on 2026-10-16 at the user's request; the brief below is historical)

### (historical) 00-first-review — "Your First Program, Reviewed"
Student's existing code at /home/csmith/Workspace/tools/src/main.go (read it; build script at /home/csmith/Workspace/tools/bin/build.sh is `go build src/main.go`, no go.mod). Verified behaviour: `go run src/main.go` works; `-n 5` panics "flag(name) not set"; `-name Bob` panics "flag(n) not set" — the flag.Visit logic is inverted (Visit only walks flags that WERE set, and each callback fails one of two mutually exclusive checks, so ANY explicit flag panics).
Review points to teach: semicolons + 2-space indent (gofmt), flag.Parse in init() (breaks `go test`, hidden side effects), inverted Visit logic, panic for user input errors (use usage + exit code 2), fmt.Println(fmt.Sprintf(...)) -> fmt.Printf, package-level mutable globals, no go.mod / file-mode builds / binary dumped in repo root (`go build -o bin/hello ./cmd/hello`), "required flag" semantics vs defaults, testable `run(args []string, stdout, stderr io.Writer) error` pattern with flag.NewFlagSet + ContinueOnError.
Lesson also covers: go modules, project layout (cmd/, internal/), go build/run/test/vet/fmt, gopls, how to read compiler errors.
Exercises: (1) `hello` — rewrite their program as a testable `Run(args, stdout, stderr) int` (or error) with FlagSet; tests check output, invalid flags, required-flag handling; (2) `fizzbuzz`-style warm-up returning []string; (3) `temperature` conversion with typed float (type Celsius float64) and String methods.
Project: `wc` clone — counts lines/words/bytes/runes from files or stdin with flags -l -w -c -m; testable core `Count(r io.Reader) (Counts, error)`; CLI wrapper tests.

## 01-basics — "Types, Slices, Maps, Strings"
Concepts: zero values, slices (backing arrays, len/cap, append aliasing, full slice expr, preallocation), maps (nil map write panic, iteration order random, comma-ok, struct{} sets), strings vs bytes vs runes, UTF-8, strings.Builder, constants/iota, slices/maps/cmp packages, min/max/clear builtins, range-over-int.
Exercises: reverse UTF-8 string by runes; dedupe slice preserving order; top-N word frequencies (deterministic tie-break); anagram check (unicode, case-insensitive); rotate/chunk a slice (aliasing awareness).
Project: "Text Analyzer" — reads text, outputs word count, unique words, top-N words with stopword list, longest word, average word length; deterministic output; tests.

## 02-errors — "Errors as Values"
Concepts: error interface, sentinel errors, custom error types, %w wrapping, errors.Is/As, errors.Join, panic/recover appropriate uses, error message style, handle-once, adding context, defer + named return for close errors.
Exercises: config `key=value` parser returning *ParseError{Line, Msg} with Unwrap; sentinel ErrNotFound in a lookup + wrapping through layers; validation with errors.Join; safe division/recover wrapper (when recover is legitimate).
Project: "Bank Ledger" — process a transaction file (deposit/withdraw/transfer), typed errors (ErrInsufficientFunds, *AccountError), report of failures without stopping, exit codes.

## 03-types-interfaces — "Structs, Methods & Interfaces"
Concepts: structs, value vs pointer receivers, method sets, embedding (and its promotion pitfalls), interfaces (implicit, small, consumer-defined), Stringer, type switches/assertions, nil interface vs nil pointer, compile-time assertions `var _ I = (*T)(nil)`, constructors NewX, functional options, sort.Slice vs slices.SortFunc.
Exercises: Shape interface with Area/Perimeter + Stringer; Stack type with pointer receivers; a type switch formatter; an embedded Logger / Animal-style composition done right; functional options for a Server config.
Project: "Inventory Manager" — Product/Inventory types, Store interface with an in-memory implementation, sorting/filtering reports, low-stock alerts, tests.

## 04-testing — "Testing Like a Gopher"
Concepts: table-driven tests, subtests, t.Helper, t.Cleanup, t.TempDir, t.Parallel (and loop var semantics since 1.22), testdata/ & golden files with -update flag, Example functions, fuzzing (go test -fuzz), benchmarks (b.Loop, -benchmem), coverage (-cover, -coverprofile, go tool cover -html), black-box `package x_test`, test doubles via interfaces (fakes over mocks), httptest preview.
Twist: here the STUDENT writes tests. Provide implementations with planted bugs (documented count, e.g. "this package has 3 bugs"); student writes tests that expose them, then fixes. Provide a starter test file with a skeleton (compiles, maybe one example case). The solution contains fixed code + complete tests. Include a fuzz exercise where fuzzing finds a real bug (e.g. a reverse/round-trip function mishandling UTF-8). Include a golden-file exercise and a benchmark exercise.
Project: "Test an untested package" — a provided small `slugify`/`markdown-to-text`/`semver` package with no tests and 3-4 planted bugs; reach ≥90% coverage, find & fix bugs, add fuzz test and benchmark.

## 05-io — "Readers, Writers, Files & Encoding"
Concepts: io.Reader/Writer, composition (io.MultiWriter, TeeReader, LimitReader, io.Copy), bufio.Scanner & token limit, bufio.Writer + Flush, os files, defer Close + checking Close on writes, os.ReadFile vs streaming, filepath.WalkDir, encoding/csv, encoding/json (tags, omitempty, Decoder streaming, DisallowUnknownFields), os.Root (1.24) mention.
Exercises: custom rot13 io.Reader; line counter on io.Reader handling long lines; CSV → JSON converter; tail last N lines (ring buffer); JSON config loader with validation.
Project: "Log Analyzer" — parse access-log lines (provide testdata), aggregate status codes, top paths, p50/p95 latency, output text or JSON (-format flag), streaming not loading whole file.

## 06-generics — "Generics & Iterators"
Concepts: type parameters, constraints, comparable, cmp.Ordered, ~ tilde, type inference, generic types, when NOT to use generics, iter.Seq/Seq2 and range-over-func (1.23), slices.Collect, maps.Keys/Values (iterators), slices.Sorted.
Exercises: Map/Filter/Reduce; Set[T comparable] with Union/Intersect and All() iter.Seq; generic Stack/Queue; SumNumbers with ~int constraint; iterator adapters (Filter/Take over iter.Seq).
Project: "Generic LRU Cache" — Cache[K comparable, V any] with Get/Put/Len, capacity eviction, optional TTL, All() iterator in recency order; (container/list readable vs custom doubly-linked/array-backed fast version).

## 07-concurrency — "Goroutines, Channels & Sync"
Concepts: goroutines, channels (buffered/unbuffered, ownership, closing rules), select, sync.WaitGroup, Mutex/RWMutex, sync.Once/OnceValue, atomic, race detector, worker pools, fan-out/fan-in, pipelines, bounded concurrency via semaphore channel, goroutine leaks, context cancellation in workers, errgroup-like pattern with stdlib.
Exercises: concurrent-safe counter (mutex vs atomic); parallel sum over chunks; worker pool processing jobs preserving result order; pipeline generator→square→sink with cancellation (no leaks — test with runtime.NumGoroutine or a done check); bounded parallel "fetcher" with a fake fetch func and max-in-flight assertion.
Project: "Duplicate File Finder" — walk directory (WalkDir), group by size, hash candidates concurrently with bounded workers (crypto/sha256), report duplicate groups; tests with t.TempDir; must pass -race.

## 08-http — "Context, HTTP Servers & Clients"
Concepts: context (cancellation, deadlines, values), net/http handlers, Go 1.22 ServeMux patterns (`GET /items/{id}`, r.PathValue), middleware chaining, JSON encode/decode helpers, status codes, log/slog structured logging, http.Server timeouts, graceful shutdown with signal.NotifyContext, http.Client with timeout, httptest.NewRecorder/NewServer.
Exercises: health handler; JSON helpers with error responses; logging + recovery middleware; client with timeout & context that calls an httptest server; path params routing.
Project: "URL Shortener API" — POST /links, GET /{code} redirect, GET /links/{code}/stats, in-memory store behind interface with mutex, validation, tests via httptest, graceful shutdown main in cmd/.

## 09-performance — "Measure, Profile, Optimize"
Concepts: benchmarks (b.Loop, -benchmem, -count, benchstat), pprof (cpu/mem, `go test -cpuprofile`, `go tool pprof -top/-list/-http`), escape analysis (-gcflags=-m), allocations, preallocation, strings.Builder, strconv vs fmt, []byte vs string, map sizing, sync.Pool caveats, struct field ordering, avoiding regexp in hot paths, bounds-check elimination, PGO mention. "Measure first", readability trade-offs.
Exercises: each provides a correct-but-slow function and a test that asserts correctness AND `testing.AllocsPerRun` ≤ target (or relative speed via benchmark guidance) — so starter tests fail until optimized: string join in loop; fmt.Sprintf int formatting → strconv.AppendInt; regex parsing → manual parse; map with prealloc; reuse buffers.
Project: "Optimize the Log Parser" — a provided slow parser + benchmark; goal: ≥5x faster and ≤ N allocs/op while tests stay green; student records before/after benchstat-style numbers in project/RESULTS.md.

## 10-capstone — "Capstone: Mini Key-Value Store" (3 milestones, each < 1h)
Milestone 1: concurrency-safe in-memory store with TTL expiry (generic not required), Get/Set/Delete/Keys, background janitor with context-based shutdown; tests with -race.
Milestone 2: persistence via append-only log (JSON lines or simple binary), replay on startup, compaction; tests with t.TempDir.
Milestone 3: HTTP API (GET/PUT/DELETE /kv/{key}), slog logging, graceful shutdown, httptest tests; cmd/kvd main.
Each milestone has its own directory with starter + tests; later milestones may build on student's earlier code but should ship their own starter so they are independent.
