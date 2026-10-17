// Package envcheck implements the envcheck command: a "hello world" that also
// reports the Go environment it runs in, so you can confirm that your host
// setup and the course container both work.
//
// The command's logic lives in Run, not in main, so tests can call it with
// any arguments and capture its output. See cmd/envcheck for the 5-line main.
package envcheck

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"

	"gocourse/solutions/00-setup/exercises/hello"
)

// Env describes the environment envcheck reports with -v. Run receives it
// as a parameter instead of looking it up itself, so tests can pass a fixed
// Env and get the same output on every machine.
type Env struct {
	GoVersion   string // e.g. "go1.24.2"
	OS, Arch    string // e.g. "linux", "amd64"
	CPUs        int
	InContainer bool
}

// DetectEnv returns the Env of the running program.
func DetectEnv() Env {
	return Env{
		GoVersion:   runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		CPUs:        runtime.NumCPU(),
		InContainer: fileExists("/.dockerenv") || fileExists("/run/.containerenv"),
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Run executes envcheck with the given command-line arguments (without the
// program name) and returns the process exit code.
//
// Output on stdout:
//
//	Hello, <name>!                  (hello.Greeting; -name defaults to "")
//	Your Go environment works.
//
// With -v, a blank line and then:
//
//	go version: <env.GoVersion>
//	platform: <env.OS>/<env.Arch>
//	cpus: <env.CPUs>
//	container: yes|no
//
// Exit codes: 0 on success (including -h, which prints usage to stderr);
// 2 for a usage error (unknown flag or any positional argument), with a
// message and the usage on stderr and nothing on stdout.
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
