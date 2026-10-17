# Project: `envcheck`, hello world, environment edition

**Time:** ≤ 30 minutes. **Prerequisite:** the `hello` exercise passes (the
project imports it).

A traditional first program prints "Hello, World!". This one does that and
then reports the environment it runs in, so you can see at a glance that your
host setup and the course container both work, and tell which one you are in.

```console
$ go run ./modules/00-setup/project/cmd/envcheck -v -name Ada
Hello, Ada!
Your Go environment works.

go version: go1.24.2
platform: linux/amd64
cpus: 8
container: no
```

## What is provided

- `cmd/envcheck/main.go`: the complete `main`. It only calls
  `envcheck.Run(os.Args[1:], envcheck.DetectEnv(), os.Stdout, os.Stderr)` and
  exits with the result. Read it, but don't change it.
- `envcheck/envcheck.go`: the `Env` type and `DetectEnv`, complete. `DetectEnv`
  reads the values from the `runtime` package and decides `InContainer` by
  looking for `/.dockerenv` (Docker) or `/run/.containerenv` (Podman).
- `envcheck/envcheck_test.go`: the tests, complete.

## What you implement

`Run(args []string, env Env, stdout, stderr io.Writer) int` in
`envcheck/envcheck.go`.

### Flags

| Flag | Meaning |
|---|---|
| `-name string` | who to greet; empty or blank means "World" (use `hello.Greeting`) |
| `-v` | also print the environment report |
| `-h` | print usage to stderr and exit 0 (the `flag` package does this for you) |

Parse them with `flag.NewFlagSet("envcheck", flag.ContinueOnError)` and
`fs.SetOutput(stderr)`, **not** the global `flag` functions.

### Output (stdout)

```
Hello, <name>!
Your Go environment works.
```

With `-v`, followed by a blank line and:

```
go version: <env.GoVersion>
platform: <env.OS>/<env.Arch>
cpus: <env.CPUs>
container: yes|no
```

Use the `env` argument, **not** `runtime` directly. That is what makes the
output testable: the tests pass a fixed `Env` and expect exact text.

### Errors and exit codes

| Situation | Exit code | stdout | stderr |
|---|---|---|---|
| success | 0 | the output above | nothing |
| `-h` | 0 | nothing | usage |
| unknown flag, or a flag missing its value | 2 | nothing | the `flag` package's message and the usage |
| any positional argument (`envcheck Ada`) | 2 | nothing | `envcheck: unexpected argument "Ada"`, then the usage (`fs.Usage()`) |

## Acceptance criteria

- [ ] `go test -race ./modules/00-setup/...` passes (exercise and project).
- [ ] `gofmt -l modules/00-setup` prints nothing; `go vet ./modules/00-setup/...` is clean.
- [ ] `Run` never calls `os.Exit`, never touches `os.Stdout`/`os.Stderr`/`os.Args`, and doesn't use the global `flag` functions.
- [ ] It works on the host **and** in the container, and `container:` differs:

```bash
go run ./modules/00-setup/project/cmd/envcheck -v
docker compose -f modules/00-setup/compose.yaml run --rm envcheck
```

## Test commands

```bash
go test ./modules/00-setup/project/...
go test -v -run TestRunUsageErrors ./modules/00-setup/project/envcheck/
docker compose -f modules/00-setup/compose.yaml run --rm test      # same, in the container
```

## Hints

- `flag.ErrHelp` is the error `Parse` returns for `-h`. Compare with
  `errors.Is`.
- After `Parse`, `fs.NArg()` and `fs.Arg(0)` give the leftover positional
  arguments.
- When `Parse` fails, the `flag` package has **already** printed the error and
  the usage to the writer you gave `SetOutput`. Just return 2.
- `fmt.Fprintln(stdout, ...)` and `fmt.Fprintf(stdout, ...)` write to any
  `io.Writer`.

## Stretch goals

1. Add `-json`: print the report as a JSON object (`encoding/json`, struct
   tags) instead of text, and add a test for it.
2. Report whether the race detector would work: build a second `Env` field
   from `go env CGO_ENABLED` by running `exec.Command("go", "env",
   "CGO_ENABLED")`. Keep it testable by passing a function, not calling
   `exec` inside `Run`.
3. Build a real binary and run it outside `go run`:
   `go build -o bin/envcheck ./modules/00-setup/project/cmd/envcheck && ./bin/envcheck -v`.
   Then check its exit code for a bad flag: `./bin/envcheck -x; echo $?`.
