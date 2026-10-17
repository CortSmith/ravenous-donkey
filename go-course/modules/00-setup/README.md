# Module 00 — Setup & First Program

Before you write any Go, you need an environment you trust: the right
toolchain, an editor that understands Go, and a one-command way to run the
tests. This module sets that up in one of two ways, or both:

- **Option A: on your host.** Install Go directly on your machine. This is the
  fastest day-to-day loop and gives the best editor integration.
- **Option B: in a container.** Run Go 1.24.2 from the course's Docker image
  with Docker Compose. Nothing is installed on your machine except Docker,
  and everyone gets exactly the same toolchain.

Then you prove that the setup works by building a small program: a greeting
function and an `envcheck` command that reports which Go, OS and CPU count
it runs on and whether it is inside a container. Run it both ways, and the
output tells you which environment you are in.

## Goals

After this module you will be able to:

- Install Go 1.24 on Linux, macOS or Windows, verify it, and explain what `GOROOT`, `GOPATH`, `GOMODCACHE`, `GOCACHE` and `GOTOOLCHAIN` do.
- Set up an editor with `gopls` so that formatting, imports and errors are handled as you type.
- Run the whole course in a container with Docker Compose: tests, `-race`, benchmarks, fuzzing, and servers with published ports.
- Explain how this course repository is organized as one Go module, and run any package's tests from the course root.
- Write a **testable `main`**: a tiny `main` that calls `Run(args, stdout, stderr)`, with flags parsed by a `flag.FlagSet` and proper exit codes.
- Use the everyday commands: `go test`, `go run`, `go build`, `go vet`, `gofmt`, `go doc`.

## Concepts

### 1. What you are installing

"Installing Go" means installing the **toolchain**, a single directory that
contains:

| Piece | What it does |
|---|---|
| `go` | The one command for everything: `build`, `run`, `test`, `vet`, `mod`, `doc`, `env`, `install`… |
| `gofmt` | The formatter. Go has exactly one code style, and `gofmt` enforces it. |
| The compiler and linker | Called by `go build`; you never run them directly. |
| The standard library source | `net/http`, `encoding/json`, `testing`… The library is the source; you can read all of it. |

Two caches appear the first time you build:

- the **build cache** (`GOCACHE`): compiled packages and test results. This is
  why the second `go test` run is instant and prints `(cached)`.
- the **module cache** (`GOMODCACHE`): downloaded third-party modules. This
  course uses only the standard library, so it stays nearly empty.

The **editor support** is a separate program, `gopls` (the Go language
server). Your editor talks to it for completion, go-to-definition, rename,
inline errors and format-on-save.

### 2. Which version

The course targets **Go 1.24** and pins **1.24.2** in its `go.mod`:

```
module gocourse

go 1.24.2
```

That `go` line is the **minimum** version that can build the module. Since Go
1.21, the `go` command manages toolchains for you (`GOTOOLCHAIN=auto` is the
default):

- **Your Go is newer** (1.26, 1.27): it builds the course as is. Go keeps
  backward compatibility, so everything here works unchanged.
- **Your Go is older but at least 1.21**: the first `go` command in the course
  directory **downloads go1.24.2 automatically** and uses it for this module.
  You need a network connection once.
- **Your Go is older than 1.21**: it refuses to build. Upgrade.

Go 1.24 is out of support as of 2026 (1.26 and 1.27 are current). The course
pins it so that every example, benchmark number and error message matches
what you see. Newer releases are mentioned in prose where they matter; see
[`docs/RESOURCES.md`](../../docs/RESOURCES.md).

### 3. Option A: install Go on your host

Download installers from **https://go.dev/dl/**. Pick go1.24.2, or a newer
release (see above).

#### Linux

The official tarball is the recommended way. Distribution packages (`apt
install golang`) are often several releases behind.

```bash
cd /tmp
curl -LO https://go.dev/dl/go1.24.2.linux-amd64.tar.gz     # arm64: go1.24.2.linux-arm64.tar.gz
sha256sum go1.24.2.linux-amd64.tar.gz                      # compare with the checksum on go.dev/dl
sudo rm -rf /usr/local/go                                  # never untar over an old install
sudo tar -C /usr/local -xzf go1.24.2.linux-amd64.tar.gz
```

Then put the `go` command, and the programs you later `go install`, on your
`PATH`. Add these lines to `~/.profile` (or `~/.bashrc` / `~/.zshrc`), then
open a new terminal:

```bash
export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"
```

#### macOS

Either run the `.pkg` installer from go.dev/dl, which installs to
`/usr/local/go` and adds it to your `PATH` through `/etc/paths.d/go`, or use
Homebrew:

```bash
brew install go        # installs the latest release, which builds the course fine
```

Add `$HOME/go/bin` to your `PATH` for tools you install with `go install`.

#### Windows

Run the `.msi` installer from go.dev/dl, or:

```powershell
winget install --id GoLang.Go
```

The installer sets up `PATH`. Open a **new** terminal afterwards. For this
course, **WSL 2 (Ubuntu) is strongly recommended**: the course scripts are
bash, Docker works best there, and file paths match every example. Inside WSL,
follow the Linux steps.

#### Several versions side by side

To try another release without replacing your main install:

```bash
go install golang.org/dl/go1.24.2@latest   # a small wrapper command
go1.24.2 download                          # fetches that toolchain into ~/sdk/
go1.24.2 version                           # use it like `go`
```

#### Verify

```bash
go version
# go version go1.24.2 linux/amd64     (or newer)

go env GOROOT GOPATH GOMODCACHE GOCACHE GOTOOLCHAIN
# /usr/local/go
# /home/you/go
# /home/you/go/pkg/mod
# /home/you/.cache/go-build
# auto
```

What these mean:

- `GOROOT`: where the toolchain lives. Don't set it yourself; `go` works it out.
- `GOPATH`: defaults to `~/go`. Today it only holds the module cache
  (`pkg/mod`) and binaries from `go install` (`bin`). **Your code does not
  go in GOPATH.** That was the pre-2018 way. Modules live anywhere, like this
  course does.
- `GOMODCACHE`, `GOCACHE`: the caches from section 1. `go clean -cache`
  empties the build cache if you ever suspect it.
- `GOTOOLCHAIN`: `auto` lets `go` switch toolchains as described in section 2.

Finally, from the course root:

```bash
go test ./modules/00-setup/...
```

It **fails**: the exercise is not written yet. Failing tests with no
build errors mean your setup works. If you see `go: command not found`, fix
your `PATH`. If you see `cannot find main module`, you are not in the course
directory.

#### Editor

Pick one; all of them use `gopls` underneath.

- **VS Code** with the official **Go** extension (`golang.go`). On first open
  it offers to install `gopls` and friends: accept. Format-on-save and
  organize-imports are on by default.
- **GoLand** (JetBrains): Go support built in.
- **Neovim / Vim / Emacs / Helix**: configure the LSP client for `gopls`.
  Install it with:

  ```bash
  go install golang.org/x/tools/gopls@latest     # lands in ~/go/bin
  ```

Whatever you choose, make sure **saving a file runs `gofmt` (or `goimports`)**.
You should never format Go code by hand.

Optional extra tools, mentioned throughout the course but not required:
`staticcheck` (a stricter `go vet`), `govulncheck` (known-vulnerability
scanner) and `golangci-lint` (a runner for many linters).

### 4. Option B: run everything in a container

#### Why (and why not)

| | Host install | Container |
|---|---|---|
| Toolchain | Whatever you installed | Exactly go1.24.2, same for everyone |
| Changes to your machine | Go in `/usr/local/go`, caches in `~` | None except Docker images and volumes |
| Speed | Fastest | First build downloads the Go image (about 850 MB on disk); then near-native on Linux, slower bind mounts on macOS/Windows |
| Editor | Native `gopls` | Needs the dev container (below) or a host `gopls` |
| Matches CI / production | Rarely exactly | Same mechanism most CI systems use |

Many people use both: the host for the edit-test loop, and the container to
check that things work on a clean, pinned toolchain.

#### Install Docker

- **Linux:** Docker Engine plus the Compose plugin (docs.docker.com/engine/install/).
  To run `docker` without `sudo`, add yourself to the `docker` group
  (`sudo usermod -aG docker $USER`, then log out and in). Be aware that
  membership of that group is equivalent to root access.
- **macOS / Windows:** Docker Desktop (Windows: enable the WSL 2 backend).

Check that you have **Compose v2**, the `docker compose` subcommand (with a
space), not the retired `docker-compose` script:

```bash
docker --version
docker compose version
```

#### The files

```
docker/
  Dockerfile            # the course image: golang:1.24.2-bookworm + cache dirs + a gopher user
  compose.base.yaml     # the shared "go" service every compose file extends
compose.yaml            # course-wide tasks: shell, verify, solutions
modules/NN-*/compose.yaml   # one per module: test, vet, bench, cover, shell, ...
.devcontainer/devcontainer.json  # optional: VS Code "Reopen in Container"
```

**`docker/Dockerfile`**, abridged:

```dockerfile
# Debian (bookworm): includes gcc, so cgo and -race work
FROM golang:1.24.2-bookworm
ENV GOCACHE=/cache/go-build \
    GOMODCACHE=/cache/mod \
    GOTOOLCHAIN=local \
    HOME=/tmp
RUN mkdir -p /cache/go-build /cache/mod /data && chmod 1777 /cache/go-build /cache/mod /data
WORKDIR /course
```

- **`bookworm`, not `alpine`.** `go test -race` needs cgo, and cgo needs a C
  compiler. The Alpine image has none, so `-race` fails there.
- **`GOTOOLCHAIN=local`** stops `go` from downloading any other toolchain:
  inside the container it is always 1.24.2.
- The **cache directories** are mounted as named volumes (next file), so the
  build cache survives between runs and the second `go test` is fast.

**`docker/compose.base.yaml`** defines one service, `go`, which the other
compose files extend:

```yaml
services:
  go:
    build: .
    image: gocourse-dev:1.24.2
    working_dir: /course
    user: "${UID:-1000}:${GID:-1000}"
    volumes:
      - ..:/course                  # the course directory, live (bind mount)
      - gomod:/cache/mod            # named volumes shared by all modules
      - gobuild:/cache/go-build
```

- The **bind mount** means the container sees your edits instantly. You edit
  on the host, and the container compiles and tests.
- **`user:`** runs the container as your user, not root, so files it creates
  (fuzz corpora, golden files, binaries) are yours. If `id -u` and `id -g`
  are not both 1000 on your machine, add this to your `~/.bashrc` (or
  `~/.zshrc`):

  ```bash
  export UID; export GID="$(id -g)"
  ```

**Each module's `compose.yaml`** extends `go` and adds that module's tasks:

| Service | Runs | Profile |
|---|---|---|
| `test` | `go test -race ./modules/NN-*/...` | (default) |
| `vet` | `gofmt -l` (must print nothing) and `go vet` | `tools` |
| `bench` | `go test -run='^$' -bench=. -benchmem` | `tools` |
| `cover` | `go test -cover` | `tools` |
| `shell` | an interactive `bash` in `/course` | `tools` |
| `solutions` | the reference solutions' tests | `instructor` |
| `envcheck` | module 00 only: your project, `-v` | `tools` |
| `fuzz` | module 04 only: one fuzz target (see that module) | `tools` |
| `kvd` | module 10 only: your capstone server on `localhost:8080` | `app` |

**Profiles** keep the extra services out of `docker compose up`.
`docker compose run <service>` starts any service by name, profile or not,
so you rarely need to think about them.

#### Using it

From the course root, point `-f` at a module's file:

```bash
docker compose -f modules/00-setup/compose.yaml run --rm test
docker compose -f modules/00-setup/compose.yaml run --rm envcheck
docker compose -f modules/00-setup/compose.yaml run --rm shell    # then: go test ./modules/00-setup/... -v
```

Or `cd` into the module and leave out `-f`:

```bash
cd modules/04-testing
docker compose run --rm test
FUZZ_TARGET=FuzzSplit docker compose run --rm fuzz
```

- **`run --rm`** starts a one-off container and deletes it when the command
  ends. Use it for everything except long-running servers.
- The **first** run builds the image (a minute or two). Later runs start in
  about a second.
- Any command works: `docker compose run --rm test go doc strings.Cut`.
  Arguments after the service name replace its default command.

Course-wide tasks use the root `compose.yaml`:

```bash
docker compose run --rm shell        # a shell with Go 1.24.2, at /course
docker compose run --rm solutions    # every reference solution, with -race
```

#### Servers in containers

Two rules matter for any server you run in a container (module 10's `kvd`
service follows both):

1. **Listen on `0.0.0.0`, not `127.0.0.1`.** Inside a container, `127.0.0.1`
   is the *container's* loopback; the published port can't reach it. The
   compose file then publishes the port only on your machine's loopback:
   `ports: ["127.0.0.1:8080:8080"]`.
2. **Make your program PID 1, and handle SIGTERM.** `docker compose stop`
   sends SIGTERM, waits (`stop_grace_period`), then sends SIGKILL. The
   service builds the binary and then `exec`s it, so the server itself, not a
   shell, receives the signal and can shut down gracefully.

```bash
cd modules/10-capstone
docker compose --profile app up kvd        # Ctrl-C (or `docker compose stop kvd`) → graceful shutdown
```

#### Dev container (optional, VS Code)

With the **Dev Containers** extension installed, open the course folder and
run **"Dev Containers: Reopen in Container"**. VS Code builds the same image
and runs its terminal, `gopls` and test runner inside it, as user `gopher`
remapped to your UID. Your editor then needs no Go install at all.

The dev container sets `GOTOOLCHAIN=auto` (the compose services keep
`local`). Current `gopls` releases need a newer Go to build, and `auto` lets
the Go extension download one just for that, while the course still compiles
with 1.24.2 because `go.mod` asks for it. To install a tool from a compose
shell, do the same thing by hand:

```bash
docker compose run --rm -e GOTOOLCHAIN=auto shell go install golang.org/x/tools/gopls@latest
```

#### Cleaning up

```bash
docker compose -f modules/10-capstone/compose.yaml --profile app down     # stop and remove containers
docker volume rm gocourse-gomod gocourse-gobuild gocourse-kvd-data        # caches and kvd data
docker image rm gocourse-dev:1.24.2
```

### 5. How this course repository works

The whole course is **one Go module** named `gocourse`, defined by the
`go.mod` at its root. A **package** is a directory of `.go` files. A
package's **import path** is the module path plus the directory:

```
go-course/                                   module gocourse
  go.mod
  modules/00-setup/exercises/hello/          import "gocourse/modules/00-setup/exercises/hello"
  modules/00-setup/project/envcheck/         import "gocourse/modules/00-setup/project/envcheck"
  modules/00-setup/project/cmd/envcheck/     package main: a command
  solutions/00-setup/...                     the same layout, solved
```

Consequences you will use every day:

- **Run commands from the course root.** `go` finds `go.mod` by walking up
  from the current directory, and the course's commands all start with
  `./modules/...`.
- **`./...` means "this directory and everything below it"**:
  `go test ./modules/00-setup/...` tests both packages in the module.
- **The package name is the last element of the import path**
  (`.../exercises/hello` declares `package hello`). Code that imports it
  writes `hello.Greeting`.
- A **command** is a `package main` with a `func main()`. By convention it
  lives under `cmd/<name>/`, which is the name `go build` gives the binary.
- **Test files** (`*_test.go`) sit next to the code they test and are only
  compiled by `go test`.

### 6. Your first program: a testable `main`

A naive Go program puts everything in `main`: it parses flags with the
global `flag` package, prints with `fmt.Println`, and exits with `os.Exit`.
That works, but **you can't test it**. `main` takes no arguments, writes to
the real terminal, and `os.Exit` would kill the test process.

The fix, used throughout this course, is to make `main` a two-line adapter
and put the program in a function you can call:

In the project, `main` lives in `cmd/envcheck/main.go` and `Run` in
`envcheck.go`. Here they share one file so you can run it. It imports *your*
`hello` package, so the greeting stays empty until you finish that exercise.

```go
import "gocourse/modules/00-setup/exercises/hello"

// Env describes the machine. The project's Env has more fields.
type Env struct{ GoVersion string }

// cmd/envcheck/main.go is just:
//
//	func main() {
//		os.Exit(envcheck.Run(os.Args[1:], envcheck.DetectEnv(), os.Stdout, os.Stderr))
//	}
//
// This main calls Run the way a test would instead.
func main() {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-name", "Ada"}, Env{}, &stdout, &stderr)
	fmt.Printf("exit %d, stdout %q\n", code, stdout.String())

	stdout.Reset()
	code = Run([]string{"-bogus"}, Env{}, &stdout, &stderr)
	fmt.Printf("exit %d, stderr starts %q\n", code, strings.SplitN(stderr.String(), "\n", 2)[0])
}

// envcheck.go
func Run(args []string, env Env, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("envcheck", flag.ContinueOnError) // return errors, don't exit
	fs.SetOutput(stderr)                                    // usage and errors go to stderr
	name := fs.String("name", "", "who to greet")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0 // -h was asked for: not an error
		}
		return 2 // usage error; flag already printed the message
	}
	fmt.Fprintln(stdout, hello.Greeting(*name))
	return 0
}
```

Why each piece:

- **`args []string`** instead of reading `os.Args`: a test passes
  `[]string{"-name", "Ada"}`.
- **`stdout, stderr io.Writer`** instead of `os.Stdout`: a test passes a
  `*bytes.Buffer` and checks exactly what was printed.
- **`flag.NewFlagSet(..., flag.ContinueOnError)`** instead of the global
  `flag.Parse()`. The global set calls `os.Exit(2)` on a bad flag and is shared
  state between tests. A local set returns an error.
- **An exit code, returned**, and `os.Exit` only in `main`. `os.Exit` skips
  deferred calls, so keep it at the very top of the program. Conventions:
  `0` success, `1` failure, `2` usage error.
- **`env Env` as a parameter**: anything that differs between machines (time,
  environment, the OS) is passed in, so tests are deterministic. Modules 03
  and 04 develop this idea further.

You will see this shape again in module 08's servers and in the module 10
capstone's `kvd`.

### 7. Everyday commands

| Command | Does |
|---|---|
| `go test ./modules/00-setup/...` | Run all tests in the module |
| `go test -run TestRun/name -v ./modules/00-setup/project/envcheck/` | One test or subtest, verbose |
| `go test -race ./...` | With the data race detector |
| `go test -count=1 ./...` | Ignore cached test results |
| `go run ./modules/00-setup/project/cmd/envcheck -v` | Build and run a command |
| `go build -o bin/envcheck ./modules/00-setup/project/cmd/envcheck` | Build a binary |
| `gofmt -l modules/00-setup` | List unformatted files (should print nothing); `-w` fixes them |
| `go vet ./modules/00-setup/...` | Catch suspicious code (wrong `Printf` verbs, copied locks, …) |
| `go doc strings.TrimSpace` | Documentation for any symbol, offline |
| `go env` | Show the toolchain's configuration |

Each works the same in the container: put `docker compose run --rm test` in
front of it (for example `docker compose run --rm test go doc strings.TrimSpace`),
or run it inside `shell`.

## Idioms & Best Practices

- **Install from go.dev, not your distro.** Old packaged versions cause
  confusing errors. Go upgrades are painless: delete `/usr/local/go`, untar
  the new one.
- **Format on save, always.** Any Go code you read was produced by `gofmt`.
  Unformatted code in a review is the first comment you'll get.
  ```bash
  gofmt -l modules/00-setup     # good: prints nothing
  ```
- **Run from the module root with `./...` paths.**
  ```bash
  go test ./modules/00-setup/...          # good
  cd modules/00-setup/exercises/hello && go test   # works, but every course command assumes the root
  ```
- **Keep `main` tiny.** It wires real dependencies (`os.Args`, `os.Stdout`, the
  real environment) into a testable `Run` and calls `os.Exit` once.
- **Use `flag.NewFlagSet` with `ContinueOnError`**, never the global `flag`
  package, in anything you want to test.
- **Write errors and usage to stderr, results to stdout**, so
  `envcheck > out.txt` captures only the result.
- **Pin toolchains where reproducibility matters**: the `go` line in `go.mod`
  for everyone, and a pinned image tag (`golang:1.24.2-bookworm`, not
  `golang:latest`) for containers.
- **Use `docker compose run --rm`** for one-off tasks, so stopped containers
  don't pile up.

## Common Pitfalls

- **`go: command not found` after installing.** `PATH` was changed in a file
  your current shell never read. Open a new terminal, or `source ~/.profile`.
- **Untarring over an old Go.** Files from both versions mix and you get odd
  compile errors. Always `rm -rf /usr/local/go` first.
- **Putting the course inside `~/go/src`.** That is the GOPATH layout from
  before modules, and it is not needed. Keep the course wherever it is.
- **`go: cannot find main module`.** You ran `go` outside the course
  directory. `cd` to the folder that contains `go.mod`.
- **"Toolchain not available" offline.** An older host Go tried to download
  go1.24.2 (section 2). Connect once, or install 1.24.2 or newer.
- **`-race requires cgo` / `C compiler "gcc" not found`.** You are on an
  Alpine image or have `CGO_ENABLED=0`. The course image is Debian-based for
  this reason. On a host, install `gcc` (on Linux, for example
  `sudo apt install gcc`).
- **Root-owned files in your checkout.** A container ran as root. The course
  compose files set `user:`, so if you see this, export `UID`/`GID`
  (section 4) or check for a custom `docker run` without `--user`.
- **`docker-compose: command not found`, or YAML errors from an old Compose.**
  Use `docker compose` (v2).
- **The server works in the container, but `curl localhost` fails.** It is
  listening on `127.0.0.1` inside the container, or the port isn't
  published. Listen on `0.0.0.0` and add `ports:`.
- **The container takes 10 seconds to stop.** The program never got SIGTERM
  (a shell was PID 1) or ignores it, so Docker waited and sent SIGKILL. `exec`
  the binary and handle the signal.
- **"Found orphan containers (gocourse-kvd-1)" warnings.** All the course's
  compose files share one project, `gocourse`, so that they share the cache
  volumes. While a server such as `kvd` runs, commands from other modules
  mention it. This is harmless. Don't add `--remove-orphans`: it would remove
  the running server. To silence it, `export COMPOSE_IGNORE_ORPHANS=true`.
- **`go install …@latest` fails in the container with "requires go >= 1.2x".**
  The image pins `GOTOOLCHAIN=local`. Prefix the command with
  `GOTOOLCHAIN=auto` (section 4, dev container).
- **Slow tests on macOS or Windows containers.** Bind mounts cross a VM
  boundary there. Keep the build cache in the named volume (the course does),
  or work on the host or in WSL 2.

## Exercises

### hello: prove the toolchain works (≈ 5 min)

`exercises/hello/hello.go` declares `Greeting(name string) string`. Implement
it so the tests pass. Read `hello_test.go` first: it is the specification
(white space handling, the default name, Unicode).

On the host:

```bash
go test ./modules/00-setup/exercises/hello/
go test -v -run 'TestGreeting/empty' ./modules/00-setup/exercises/hello/   # one subtest
```

In the container:

```bash
docker compose -f modules/00-setup/compose.yaml run --rm test
```

Also notice `ExampleGreeting` in the test file. Its `// Output:` comment is
checked by `go test`, and it appears in `go doc` output as an example.

## Project

**`envcheck`: hello world, environment edition (≤ 30 min).** Implement
`Run` in `project/envcheck/envcheck.go`: a command that greets you (using
your `hello.Greeting`) and, with `-v`, reports the Go version, platform, CPU
count and whether it is running in a container. `DetectEnv` and `main` are
provided. The spec and acceptance criteria are in
[project/README.md](project/README.md).

When it passes, run it both ways and compare:

```bash
go run ./modules/00-setup/project/cmd/envcheck -v                     # container: no
docker compose -f modules/00-setup/compose.yaml run --rm envcheck     # container: yes
```

## Submitting for Review

When the tests pass, on the host or in the container, ask your instructor
(Claude) to review `modules/00-setup/`. Mention which setup you chose (host,
container or both) and paste the output of `envcheck -v`, so the review can
check your environment too. Before asking:

```bash
gofmt -l modules/00-setup        # must print nothing
go vet ./modules/00-setup/...
go test -race ./modules/00-setup/...
```

## Further Reading

- [Download and install Go](https://go.dev/doc/install): official instructions for every OS.
- [All releases](https://go.dev/dl/) and [release notes](https://go.dev/doc/devel/release).
- [Go toolchains](https://go.dev/doc/toolchain): how `GOTOOLCHAIN` and the `go` line work.
- [Tutorial: Get started with Go](https://go.dev/doc/tutorial/getting-started) and [How to Write Go Code](https://go.dev/doc/code): modules, packages, `go install`.
- [`gopls` documentation](https://go.dev/gopls/) and the [VS Code Go extension](https://github.com/golang/vscode-go).
- [`flag` package](https://pkg.go.dev/flag): `FlagSet`, `ContinueOnError`, `ErrHelp`.
- [Official `golang` Docker image](https://hub.docker.com/_/golang): tags and variants.
- [Docker Compose file reference](https://docs.docker.com/reference/compose-file/), including [`extends`](https://docs.docker.com/compose/how-tos/multiple-compose-files/extends/) and [profiles](https://docs.docker.com/compose/how-tos/profiles/).
- [VS Code Dev Containers](https://code.visualstudio.com/docs/devcontainers/containers).
