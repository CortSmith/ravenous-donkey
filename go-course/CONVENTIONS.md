# Course Authoring Conventions

This file defines how every module in this course is laid out. Module authors follow it exactly.

## Toolchain
- Module path: `gocourse` (see `go.mod`), Go **1.24** (installed: go1.24.2). Do not use features newer than 1.24.
- **Standard library only.** No third-party modules, no `go get`. (Mention useful external tools such as `staticcheck`/`golangci-lint` in prose only.)
- All code must be `gofmt`-clean and pass `go vet`.

## Layout per module

```
modules/NN-slug/
  README.md            # the lesson (see below)
  compose.yaml         # container tasks for this module: extends docker/compose.base.yaml (see below)
  exercises/<pkg>/     # one dir per exercise; the student edits these
    <pkg>.go           # starter code: signatures, doc comments, TODOs, bodies that compile but are wrong
    <pkg>_test.go      # tests the student must make pass (written by the author, complete)
  project/             # the module's ~45-60 minute project
    README.md          # spec, requirements, acceptance criteria, stretch goals
    ... starter files + tests (same rules as exercises)
solutions/NN-slug/     # mirrors modules/NN-slug/ exactly (same sub-paths, same package names)
  exercises/<pkg>/<pkg>.go        # READABLE reference solution
  exercises/<pkg>/<pkg>_test.go   # identical copy of the student test file
  exercises/<pkg>/fast.go         # OPTIONAL: performance-tuned variant as differently-named funcs (e.g. WordCountFast) + benchmarks in fast_test.go
  project/...                     # reference solution for the project
  REVIEW.md                       # see below
```

- Starter code MUST compile (`go vet ./modules/...` passes) and its tests MUST FAIL until the student implements it. Use stub bodies such as `return nil, errors.New("not implemented")` or zero values — never `panic` in a way that kills the whole test binary unless the test is isolated. Prefer zero-value returns.
- `go test ./solutions/...` MUST pass completely, including `-race` for any concurrency module.
- Every module has a `compose.yaml` (project `name: gocourse`, shared by all modules) whose services extend the `go` service in `docker/compose.base.yaml`: at least `test` (default, `go test -race ./modules/NN-slug/...`), and `vet`, `bench`, `cover`, `shell` (profile `tools`) and `solutions` (profile `instructor`). It declares the shared `gocourse-gomod`/`gocourse-gobuild` volumes. Servers listen on `0.0.0.0`, publish on `127.0.0.1:<port>`, and run via `exec` so they receive SIGTERM. The container image pins `golang:1.24.2-bookworm` (Debian, so `-race` works).
- Exercises are small (5-20 min each); 3-5 exercises per module plus one project (≤ 60 min).
- Commands are run from the course root, e.g. `go test ./modules/03-structs-interfaces/exercises/shapes/`.

## Module README.md (the lesson)
Sections, in order:
1. **Goals** — what you'll be able to do.
2. **Concepts** — concise but in-depth teaching with runnable code snippets. Explain the *why*. Cite the Go spec / Effective Go / Go Code Review Comments / go.dev blog where relevant (links).
3. **Idioms & Best Practices** — bullet list, each with a short good/bad example.
4. **Common Pitfalls** — gotchas specific to this topic.
5. **Exercises** — list each exercise, what to implement, the exact `go test` command.
6. **Project** — one-paragraph pointer to `project/README.md`.
7. **Submitting for Review** — "When tests pass, ask your instructor (Claude) to review `modules/NN-slug/`."
8. **Further Reading** — links.

## solutions/NN-slug/REVIEW.md (instructor rubric)
For each exercise and the project:
- **Readable solution** — what it does and why it's idiomatic.
- **Optimized solution** — less-readable but faster/fewer-allocation approach, with a benchmark or reasoning (allocations, complexity, cache behavior). Show both versions side by side when not in fast.go.
- **Trade-off** — when you'd choose each.
- **Review checklist** — specific best-practice violations a student is likely to commit here (naming, error handling, receiver choice, exported API, etc.) so the instructor can grade consistently.

## Style rules for all course code
- Follow Effective Go and Go Code Review Comments: MixedCaps, short receiver names, doc comments starting with the identifier name, errors lowercase without punctuation, `%w` wrapping, accept interfaces / return concrete types, no naked returns in long funcs, no `init()` side effects.
- Table-driven tests with `t.Run` subtests, `t.Helper()` in helpers, `got/want` messages: `t.Errorf("Foo(%q) = %v, want %v", in, got, want)`.
- Benchmarks use Go 1.24's `for b.Loop() { ... }` style.
