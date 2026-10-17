# HANDOFF — Go Course Build (load this into a new Claude session)

Last updated: 2026-10-15 (late). **STATUS: course COMPLETE — Tasks A–D done.** Only open items: see "Open items for the user" below. Start by reading this file and nothing else. Load the other files only when a task needs them.

## 1. What the user wants
- The user is learning Go. Claude acts as **instructor**: gives feedback, code review, examples, tests, and projects that take **under 1 hour**.
- Reviews should point out best-practice violations. They should also show **two alternatives**: a readable solution and a faster but less readable one.
- The user asked for an in-depth, complete, accurate learning program. They asked Claude to use subagents for each section and for research, to keep the main context clean, and not to stop until the program is complete.
- **Model preferences:** subagents use **Haiku**, and the main session model is **Sonnet**. Both are set in `/home/csmith/Workspace/tools/.claude/settings.local.json` (`"model": "sonnet"`, `env.CLAUDE_CODE_SUBAGENT_MODEL=haiku`). Pass `model: "haiku"` explicitly on Agent calls.
- The user rejected the last tool call, a bug-verification script for module 04. **Ask before running it again.**

## 2. Where things are
- Course root: `/home/csmith/Workspace/tools/go-course`. Go module `gocourse`, go1.24.2 installed, **stdlib only**.
- `README.md`: syllabus, with a table of modules 00–10. `CONVENTIONS.md`: layout and style rules every module must follow (read it before editing a module).
- `modules/NN-slug/`: student-facing. Contains `README.md` (the lesson), `exercises/<pkg>/` (starter code + tests), and `project/` (with its own README).
- `solutions/NN-slug/`: mirrors the module, plus `REVIEW.md` (readable vs. optimized solutions, trade-offs, grading checklist).
- Module 10 uses `milestone1/2/3/` instead of exercises/project. M2 and M3 starters ship working code from the earlier milestones (I copied it in from solutions), so each milestone stands alone.
- `docs/RESOURCES.md`: student-facing links, tooling, and what's new after Go 1.24 (1.24 is out of support; 1.26 and 1.27 are current).
- `.authoring/`: research notes (`research-features.md`, `research-idioms.md`), `module-briefs.md` (the spec for each module), and the agent prompts (`author-prompt.md`, `resume-prompt.md`, `audit-prompt.md`).
- `scripts-verify.sh [module...]`: health check. Clean output is only the README/REVIEW line counts. Any other line is a problem. Module 04 is special: its starters are *supposed* to pass (skeleton tests), so "STARTER PASSES" is expected there.
- **The user's own code is `/home/csmith/Workspace/tools/src/main.go`. NEVER modify it.** The user is actively editing it; their version has a `requiredFlags` func.

## 3. Status per module

| Module | Built | Mechanical checks | Quality audit |
|---|---|---|---|
| 00-setup | ✅ | ✅ | ✅ 2026-10-16: REPLACED old 00-first-review (user request). Lesson = host install + Docker/Compose setup + testable main (§6); exercise `hello`, project `envcheck` |
| 01-basics | ✅ | ✅ | ✅ done (benchmarks re-measured) |
| 02-errors | ✅ | ✅ | ✅ done (README 372→669 lines) |
| 03-types-interfaces | ✅ | ✅ | ✅ done |
| 04-testing | ✅ | ✅ | ✅ done 2026-10-15 (17 bugs mutation-verified; README 668, REVIEW 478; fast.go for csvline/report/semver) |
| 05-io | ✅ | ✅ | ✅ done |
| 06-generics | ✅ | ✅ | ✅ done 2026-10-15 (benchmarks re-measured, wrong explanations fixed) |
| 07-concurrency | ✅ | ✅ | ✅ done (I reverted a wrong "loop var capture fix" in pipeline.go Merge) |
| 08-http | ✅ | ✅ | ✅ done 2026-10-15 (README fixes: uuid→rand.Text, Recover sw, etc; REVIEW fast-variant sections rewritten with measurements) |
| 09-performance | ✅ | ✅ | ✅ done (I replaced fabricated pprof / -gcflags=-m output in README with real output) |
| 10-capstone | ✅ | ✅ | ✅ done 2026-10-15 (kvd main rewritten as run() + SIGTERM + tests mirrored; REVIEW numbers measured; Goals/Exercises/Project sections) |

## 4. Remaining work (in order)

### Task A — DONE (2026-10-15). Only open item: user must delete `modules/04-testing/exercises/csvline/testdata/` (leaks fuzz crashers; deletion was denied by auto-mode) and the stray `go-course/project.test` binary.
### (old notes) Task A — Finish module 04-testing
In this module **the student writes the tests**. The starter `.go` files contain planted bugs, and the starter `*_test.go` files are skeletons.
- Done: the starter tests are now skeletons (28–43 lines). Full tests are in `solutions/04-testing/**`.
- **Problem 1:** the full solution tests were never proven to catch the planted bugs. Before the fix, the semver starter PASSED the full solution tests, so its bugs may not be real. To verify each package (csvline, concat, paginate, report, reminder, project/semver):
  1. Copy the starter non-test `.go` files, the solution `*_test.go` files and `testdata/` into a scratch dir with `go.mod` (`module bugcheck` / `go 1.24`).
  2. Run `go test ./...`. It must FAIL, and each documented bug must be caught by a named test.
  3. For csvline, also confirm `go test -run=^$ -fuzz=FuzzRoundTrip -fuzztime=20s` (and FuzzSplit) finds the bug.
  4. Fix the starter or the solution tests where a bug isn't real or isn't caught.
  5. (Ask the user before running this; they rejected it once.)
- **Problem 2:** `solutions/04-testing/REVIEW.md` shrank to 100 lines and `modules/04-testing/README.md` to 379 lines (it was reported at about 1300). Content was probably lost.
  - Rebuild REVIEW.md per the CONVENTIONS rubric. List every planted bug with the exact test that catches it.
  - Rebuild or deepen the lesson README: 8 required sections, ≥ 500 lines. It should state the **bug count** per package but not the bug details.
- Bug list the agent reported (unverified): csvline 2 (quote check in Join; unsafe slicing in Split), paginate 3, concat 2, reminder 3, report 3, semver 4.

### Task B — DONE 2026-10-15
### (old) Task B — Finish the 06 and 08 audits
For each module, run `./scripts-verify.sh 06-generics 08-http`.
- **06:** re-measure the speedup claims in REVIEW.md (MapFast 5–10%, RingQueue 2x, FastCache 2x) with `go test -run=^$ -bench=. -benchmem ./solutions/06-generics/...`. Replace any that are wrong and note "measured on go1.24.2 linux/amd64".
- **08:** check that README ServeMux examples (`GET /items/{id}`, `r.PathValue`) are correct and that every exercise and project `go test` command in the README works.

### Task C — DONE 2026-10-15
### (old) Task C — Audit 10-capstone
Follow `.authoring/audit-prompt.md` with `<m>=10-capstone`. Also check:
- M1 TTL tests use an injectable clock.
- M2/M3 READMEs mention the "Provided for you" files (I added notes).
- The M3 `cmd/kvd` main does graceful shutdown.
- REVIEW.md covers all 3 milestones.

### Task D — DONE 2026-10-15 (all verify/gofmt/vet/-race clean; root syllabus now lists exercises)
### (old) Task D — Final whole-course pass
1. `./scripts-verify.sh` (all modules), `gofmt -l modules solutions`, `go vet ./...`, `go test -race ./solutions/...`.
2. Make sure the root `README.md` syllabus matches the actual exercise and project names. Optionally list the exercises per module.
3. Decide whether to delete `scripts-verify.sh` and `.authoring/` or keep them (they're useful to the instructor). Mention `HANDOFF.md` to the user.
4. Give the user a short summary and how to start: `cat modules/00-setup/README.md`.

## 4a. Container setup (added 2026-10-16)
- `docker/Dockerfile` (golang:1.24.2-bookworm, GOTOOLCHAIN=local, /cache + /data 1777, user gopher uid 1000), `docker/compose.base.yaml` (service `go`, bind `..:/course`, volumes gocourse-gomod/gocourse-gobuild, `user: ${UID:-1000}:${GID:-1000}`).
- Root `compose.yaml` (shell; verify/solutions in profile instructor). `modules/NN/compose.yaml` per module, all `name: gocourse`: test (default), vet/bench/cover/shell (tools), solutions (instructor); 00 envcheck, 04 fuzz (FUZZ_PKG/FUZZ_TARGET/FUZZ_TIME), 10 kvd (profile app, :8080) + kvd-solution (instructor, :8081).
- `.devcontainer/devcontainer.json` (GOTOOLCHAIN=auto so gopls can build) — not tested (no devcontainer CLI).
- `scripts-verify.sh` also runs `docker compose config -q` per module when docker exists. `index.html` is generated by `scripts-build-site.py` from `.authoring/site-template.html` — rebuild after content changes.
- `cmd/runner` (added 2026-10-17): stdlib-only local server that serves the course site and runs lesson snippets (`go run ./cmd/runner` → http://localhost:8090, or the `runner` service in the root `compose.yaml`). It wraps fragments (top-level func/type → package level, rest → main), auto-adds stdlib imports, and inserts `_ = x` for unused vars. Security: loopback Host check plus same-Origin + JSON content-type on POST. Tests: `go test ./cmd/runner/`. The Run UI lives in `.authoring/site-template.html` (`makeRunnable`). 2026-10-17: every ```go block in every README/REVIEW was rewritten to run (354 blocks). Intentional failures are tagged ```go expect=compile-error / expect=runtime-error. Check with `go run ./cmd/runner -check -strict [paths]` (also in scripts-verify.sh); rules are in CONVENTIONS.md "Code blocks in Markdown". Open item: solutions/09-performance/REVIEW.md prose for the project describes a regex-based logstats.go and a project fast.go that don't match the actual solution files (the code blocks run, the text is stale); several other REVIEW blocks were re-synced from the solution .go files.

## 4b. Open items for the user (cannot be done by Claude in auto mode; deletions were denied)
- Delete `modules/04-testing/exercises/csvline/testdata/` (ships the fuzz crashers → spoils the fuzzing exercise).
- Delete stray binary `go-course/project.test` (4 MB test binary left by an earlier agent).
- Kept on purpose: `scripts-verify.sh`, `.authoring/` (instructor tooling), this file.

## 5. Lessons learned (important for the next session)
- **Haiku subagent reports are unreliable.** They claimed "all checks pass" when starters didn't compile (02). They copied the full solution tests into module 04 starters, leaking the answers. They invented benchmark numbers and pprof output (01, 06, 09). They made a wrong "fix" (07 loop-var capture; that's not a bug since Go 1.22). One agent also falsely claimed it had modified the user's `src/main.go`.
  - **Always run `./scripts-verify.sh <m>` yourself after an agent finishes**, and spot-check its claims.
- Give agents exact verification commands and ask them to paste real output.
- Rate limits hit when about 10 agents run at once. Use smaller batches.
- Code style the course enforces: gofmt, `run(args, stdout, stderr)` testable main, no `init()` side effects, `%w` wrapping, table-driven tests, `b.Loop()` benchmarks (1.24). No features newer than 1.24 in code; Go 1.25+ features may be mentioned only in prose.

## 6. Pending feedback for the user (already given in chat)
On the user's edited `src/main.go`:
- Their `requiredFlags` correctly fixes the inverted `flag.Visit` logic and correctly exits with code 2 plus usage.
- **But:** `-input`/`-output`/`-verbose` are defined *after* `flag.Parse()`, so they're never parsed, and passing them makes Parse error out.
- Making flags required while also giving them defaults is contradictory.
- `Parse` still runs in `init()`, and semicolons and 2-space indentation remain (run gofmt).
- `run()` is empty. (The old module 00 `hello` exercise that covered this fix was removed on 2026-10-16 when the user replaced module 00 with setup; module 00 §6 still teaches the testable-main pattern.)
