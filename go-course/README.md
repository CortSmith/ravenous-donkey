# Go, Hands-On: An Instructor-Led Course

A self-paced but **instructor-reviewed** Go course. Each module has an in-depth lesson, a few short exercises (5-20 min each) with tests already written, and a project you can finish in under an hour. When you finish one, ask Claude to review it. You'll get feedback on best-practice violations, plus two alternatives for comparison: a **readable** solution and a **faster but less readable** one.

## How to work through it

**Start with [module 00](modules/00-setup/)**: it sets up Go on your machine
(Option A) or in a Docker container (Option B). Either way works for the whole
course.

On the host:

```bash
cd go-course
cat modules/01-basics/README.md               # 1. read the lesson
$EDITOR modules/01-basics/exercises/…         # 2. implement the stubs
go test ./modules/01-basics/...               # 3. make the tests pass
go vet ./modules/01-basics/... && gofmt -l modules/01-basics
```

Or in the container. Every module has a `compose.yaml`:

```bash
docker compose -f modules/01-basics/compose.yaml run --rm test    # tests, with -race
docker compose -f modules/01-basics/compose.yaml run --rm vet     # gofmt + go vet
docker compose -f modules/01-basics/compose.yaml run --rm shell   # a shell with Go 1.24.2
```

4. Ask for a review: *"Review my work in modules/03-types-interfaces/"*. Passing tests is the minimum bar. The review looks at idiom, API design, error handling, naming, allocations and test quality.
5. Then compare your work with `solutions/<module>/`. **Don't open it before you've attempted the module.** Each solutions folder has a `REVIEW.md` with the readable vs. optimized discussion.

Useful flags: `go test -run TestName -v`, `go test -race`, `go test -bench=. -benchmem`, `go test -cover`.

To browse the course as a web page with navigation and progress tracking, open
`index.html` in your browser. Rebuild it after course changes with
`python3 scripts-build-site.py`.

**Run the lesson snippets in the page.** Start the local runner from the course root and open
<http://localhost:8090/>:

```sh
go run ./cmd/runner                                  # Go on your host
docker compose run --rm --service-ports runner       # or in the course container
```

Every Go code block then gets a **▶ Run** button. Click a block to edit it, and press Ctrl+Enter to run it.
Snippets are often fragments, so the runner wraps them in `package main` / `func main` and adds missing
standard-library imports. The page shows the exact program that ran. Every block runs as written. A few are
*meant* to fail (a compile error, a panic, a deadlock) and are labelled that way on the page. Some project
snippets import your own package, so their output changes as you finish the exercise. The runner listens on
localhost only and accepts runs only from its own page.

## Syllabus

| # | Module | Topics | Exercises (`exercises/`) | Project (≤ 60 min) |
|---|--------|--------|-----------|--------------------|
| 00 | [Setup & First Program](modules/00-setup/) | installing Go (host), Docker + Compose (container), editor/`gopls`, how the course module works, testable `main` with `flag.FlagSet` | `hello` | `envcheck`: hello world + environment report |
| 01 | [Basics](modules/01-basics/) | slices, maps, strings/runes/UTF-8, `slices`/`maps` pkgs | `reverse`, `dedupe`, `wordfreq`, `anagram`, `chunk` | Text Analyzer (`textstats`) |
| 02 | [Errors](modules/02-errors/) | sentinel/typed errors, `%w`, `errors.Is/As/Join`, panic vs error | `configparse`, `lookup`, `validate`, `safediv`, `closeerr` | Bank Ledger (`ledger`) |
| 03 | [Types & Interfaces](modules/03-types-interfaces/) | methods, receivers, embedding, small interfaces, options | `shapes`, `stack`, `describe`, `logging`, `server` | Inventory Manager |
| 04 | [Testing](modules/04-testing/) | *you* write the tests: table tests, fakes, golden files, fuzzing, examples, benchmarks, coverage | `csvline`, `paginate`, `concat`, `reminder`, `report` (each with planted bugs) | SemVer parser with 4 planted bugs (`semver`) |
| 05 | [I/O](modules/05-io/) | `io.Reader/Writer`, `bufio`, files, CSV, JSON | `rot13`, `linecount`, `csvjson`, `tail`, `config` | Log Analyzer (`loganalyzer`) |
| 06 | [Generics & Iterators](modules/06-generics/) | type params, constraints, `iter.Seq`, range-over-func | `funcs`, `set`, `stackqueue`, `numeric`, `iterx` | Generic LRU Cache |
| 07 | [Concurrency](modules/07-concurrency/) | goroutines, channels, `sync`, worker pools, pipelines, leaks, `-race` | `counter`, `parallelsum`, `workerpool`, `pipeline`, `fetcher` | Duplicate File Finder (`dupfind`) |
| 08 | [Context & HTTP](modules/08-http/) | `context`, 1.22 `ServeMux` routing, middleware, `slog`, `httptest`, graceful shutdown | `health`, `jsonio`, `middleware`, `routes`, `client` | URL Shortener API (`shortener`) |
| 09 | [Performance](modules/09-performance/) | benchmarks, `pprof`, escape analysis, allocations | `appendints`, `dedupe`, `lineenc`, `csvline`, `semver` | Optimize the Log Parser (`logstats`) |
| 10 | [Capstone](modules/10-capstone/) | everything | the three milestones | Mini Key-Value Store `kvd` (3 × ≤1h milestones) |

Suggested pace: one module per sitting or two. Module 00 is setup (under an hour). Modules 01–03 are foundations. 04 (Testing) assumes 02 (`errors.Is`) and 03 (interfaces, for fakes). 05 and 06 can be taken in either order. 07–10 build on everything before them.

## Toolchain note

The course targets **Go 1.24** (pinned to `go1.24.2` in `go.mod` and in the container image) and uses only the standard library. As of October 2026, Go 1.24 is out of support; 1.26 and 1.27 are current. Everything here also works on newer versions, and an older Go (1.21+) downloads 1.24.2 automatically. Module 00 explains both setups; [`docs/RESOURCES.md`](docs/RESOURCES.md) lists the newer features worth knowing about.

Container files: [`docker/`](docker/) (image and the shared compose service), [`compose.yaml`](compose.yaml) (course-wide tasks), `modules/NN-*/compose.yaml` (per module) and [`.devcontainer/`](.devcontainer/) (VS Code).

## Reference

- [`docs/RESOURCES.md`](docs/RESOURCES.md): style guides, official docs, tooling install commands, what's new since 1.24.
- [`CONVENTIONS.md`](CONVENTIONS.md): how modules are laid out, and the style rules the course code follows (and that reviews hold you to).
