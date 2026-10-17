// Package envcheck implements the envcheck command: a "hello world" that also
// reports the Go environment it runs in, so you can confirm that your host
// setup and the course container both work.
//
// The command's logic lives in Run, not in main, so tests can call it with
// any arguments and capture its output. See cmd/envcheck for the 5-line main.
package envcheck

import (
	"io"
	"os"
	"runtime"

	"gocourse/modules/00-setup/exercises/hello"
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
	// TODO: implement Run.
	//   1. flag.NewFlagSet("envcheck", flag.ContinueOnError), output to stderr.
	//   2. Define -name (string) and -v (bool); Parse(args).
	//   3. flag.ErrHelp means -h: return 0. Any other parse error: return 2.
	//   4. Reject positional arguments (fs.NArg() > 0) with a message + usage.
	//   5. Print the greeting with hello.Greeting, then the -v report.
	_ = hello.Greeting
	_, _, _, _ = args, env, stdout, stderr
	return 1
}
