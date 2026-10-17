# Module 00 — Setup & First Program: Instructor Review Guide

This module's goal is a **working environment**, not clever code. Review in
this order:

1. **The environment.** Ask for the output of `envcheck -v` on the host and
   in the container (`docker compose -f modules/00-setup/compose.yaml run --rm envcheck`).
   Check: `go version` is 1.24.2 or newer, `container:` is `no` on the host and
   `yes` in the container, and `gofmt -l` and `go vet` are clean (which proves
   the editor formats on save).
2. **The code**, against the checklists below.

Benchmark numbers are from go1.24.2 linux/amd64 (20 logical CPUs),
`-count=5`, medians, rounded.

---

## Exercise: `hello`

### Readable solution

```go
func Greeting(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "World"
	}
	return "Hello, " + name + "!"
}

fmt.Printf("%q %q %q\n", Greeting("Ada"), Greeting("  Ada \n"), Greeting("\t")) // "Hello, Ada!" "Hello, Ada!" "Hello, World!"
```

`strings.TrimSpace` handles spaces, tabs and newlines (any Unicode white
space), so `" \t "` and `""` share one branch. The function reassigns its
parameter instead of introducing a second variable, a common and accepted Go
idiom for normalizing input.

### Optimized solution

There is little to optimize. `+` on three operands compiles to a single
`runtime.concatstrings` call, so one allocation. `fast.go` compares the
alternatives people write:

| Variant (`fast.go`) | ns/op | B/op | allocs/op |
|---|---|---|---|
| `Greeting` (`+`) | ~37 | 24 | 1 |
| `GreetingBuilder` (pre-sized `strings.Builder`) | ~33 | 24 | 1 |
| `GreetingSprintf` (`fmt.Sprintf("Hello, %s!", name)`) | ~89 | 40 | 2 |

```bash
go test ./solutions/00-setup/exercises/hello/ -run='^$' -bench=. -benchmem
```

### Trade-off

`+` is the right answer: as clear as `Sprintf` and within ~10% of the
hand-built `Builder`. `Sprintf` costs about 2.4× more time and an extra
allocation (the argument is boxed into an `interface{}`). That is irrelevant
for one greeting but a habit worth noticing in hot loops. **Don't ask a student
to change `Sprintf` here.** Mention the cost as information.

### Review checklist

- [ ] Uses `strings.TrimSpace`, not a hand-written loop or `strings.Trim(name, " ")` (which misses tabs and newlines).
- [ ] Tests pass the `only white space` and `tabs and newline` cases. Those catch `Trim(" ")`.
- [ ] Doc comment intact and starting with `Greeting`.
- [ ] No `fmt.Println` debugging left behind.
- [ ] gofmt-clean. If not, the editor isn't formatting on save: fix the setup, not just the file.

---

## Project: `envcheck`

### Readable solution

`solutions/00-setup/project/envcheck/envcheck.go`:

```go
import "gocourse/solutions/00-setup/exercises/hello"

type Env struct {
	GoVersion   string // e.g. "go1.24.2"
	OS, Arch    string // e.g. "linux", "amd64"
	CPUs        int
	InContainer bool
}

func main() { // try a few argument lists
	env := Env{GoVersion: "go1.24.2", OS: "linux", Arch: "amd64", CPUs: 8}
	for _, args := range [][]string{{"-name", "Ada"}, {"-v"}, {"extra"}} {
		fmt.Printf("$ envcheck %s\n", strings.Join(args, " "))
		fmt.Printf("(exit %d)\n\n", Run(args, env, os.Stdout, os.Stdout))
	}
}

func Run(args []string, env Env, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("envcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "who to greet (default: World)")
	verbose := fs.Bool("v", false, "also report the Go environment")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2 // the flag package already printed the error and usage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "envcheck: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return 2
	}

	fmt.Fprintln(stdout, hello.Greeting(*name))
	fmt.Fprintln(stdout, "Your Go environment works.")
	if *verbose {
		container := "no"
		if env.InContainer {
			container = "yes"
		}
		fmt.Fprintf(stdout, "\ngo version: %s\nplatform: %s/%s\ncpus: %d\ncontainer: %s\n",
			env.GoVersion, env.OS, env.Arch, env.CPUs, container)
	}
	return 0
}
```

Points to look for:

- **Dependency injection at the edges.** `args`, `stdout`, `stderr` and
  `env` come in as parameters, and `main` supplies the real ones. This is
  why `TestRun` can check exact output on any machine, including CI and the
  container, where `cpus` and `container:` differ.
- **Usage errors return before anything reaches stdout**, so a script
  piping `envcheck` never sees half a result.
- **Reuse** of `hello.Greeting` through its import path
  `gocourse/modules/00-setup/exercises/hello`: the first taste of how packages
  in one module use each other.

### Optimized solution

None is warranted, and it's worth saying so explicitly in review. `Run` does
a handful of writes once per process. Process start-up (~1 ms) dwarfs it by
four orders of magnitude, so any "optimization" (a single `Fprintf`, a
`bufio.Writer`) only costs readability. The one performance-adjacent habit to
reinforce is **building the output with `Fprint*` to the injected writer**,
rather than concatenating a big string and printing it at the end. It is
clearer, and it streams for large outputs.

### Trade-off

The only real design decision is **how much to inject**. Passing `Env`
makes the output testable; the alternative (calling `runtime.Version()`
inside `Run`) forces tests to compute expected values with the same calls,
which tests nothing. The cost is one extra parameter. Go code makes this
trade almost always: dependencies at the edges, as plain parameters or small
interfaces.

### Review checklist

- [ ] `main` unchanged: one `os.Exit(envcheck.Run(...))` line.
- [ ] `Run` uses `flag.NewFlagSet(..., flag.ContinueOnError)` + `SetOutput(stderr)`; **no** global `flag.String`/`flag.Parse`.
- [ ] `flag.ErrHelp` checked with `errors.Is` and mapped to exit 0.
- [ ] Parse errors return 2 *without* printing the error a second time (the `flag` package already did).
- [ ] Positional arguments rejected with the exact message, followed by `fs.Usage()`.
- [ ] Uses the `env` parameter, not `runtime.*` or `DetectEnv()`, inside `Run`.
- [ ] No `os.Exit`, `os.Stdout`, `os.Stderr`, `log.Fatal` or `panic` inside `Run`.
- [ ] Output to `stdout`, diagnostics to `stderr`.
- [ ] Environment verified both ways (host `container: no`, compose `container: yes`).

### Common student mistakes

| Mistake | Symptom | Comment to give |
|---|---|---|
| Global `flag.String` + `flag.Parse()` | Tests panic "flag redefined" or exit the test binary | "The global FlagSet is process-wide state, and on error it calls `os.Exit(2)`. Use a local `FlagSet`." |
| `fmt.Println` instead of `Fprintln(stdout, …)` | `TestRun` sees empty stdout | "Write to the writer you were given. That's what lets the test capture it." |
| Printing `err` after a failed `Parse` | Error message appears twice in stderr | "`flag` already reported it to `stderr`." |
| `return 1` for usage errors | `TestRunUsageErrors` fails | "Convention: 2 means 'you called me wrong', 1 means 'I failed'." |
| Calling `DetectEnv()` inside `Run` | `TestRun` "verbose" cases fail on most machines | "That's why `env` is a parameter: tests decide the environment." |

---

## Environment review checklist (both setups)

- [ ] `go version` is 1.24.2 or newer on the host (or the student uses only the container).
- [ ] The student runs commands from the course root with `./modules/...` paths.
- [ ] Editor formats on save (no gofmt findings, imports grouped by the tool).
- [ ] Container: `docker compose ... run --rm test` passes; files created in the checkout are owned by the student, not root.
- [ ] They can explain why the image is `bookworm` and not `alpine` (`-race` needs cgo).
