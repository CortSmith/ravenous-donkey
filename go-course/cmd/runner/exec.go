package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// maxOutput caps the combined stdout+stderr kept from one run.
const maxOutput = 64 << 10

// result is the JSON reply for one Run.
type result struct {
	Program  string   `json:"program"`  // the file that was compiled
	Mode     string   `json:"mode"`     // "run" or "test"
	Phase    string   `json:"phase"`    // "build" (compile failed) or "run"
	Output   string   `json:"output"`   // compiler errors, or program output
	Exit     int      `json:"exit"`     // exit code of the program
	TimedOut bool     `json:"timedOut"` // killed after the time limit
	Ms       int64    `json:"ms"`       // run time, excluding the build
	Notes    []string `json:"notes"`    // automatic changes made to the snippet
}

type executor struct {
	root    string // course root; its packages are importable as gocourse/...
	timeout time.Duration
	slots   chan struct{} // limits concurrent runs
}

// Run wraps code into a program, builds it (repairing what it can), and
// runs it with a time limit.
func (e *executor) Run(ctx context.Context, code string) result {
	select {
	case e.slots <- struct{}{}:
		defer func() { <-e.slots }()
	case <-ctx.Done():
		return result{Phase: "build", Output: "cancelled"}
	}

	dir, err := os.MkdirTemp("", "gocourse-run-")
	if err != nil {
		return result{Phase: "build", Output: err.Error()}
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(e.goMod()), 0o644); err != nil {
		return result{Phase: "build", Output: err.Error()}
	}

	s := parseSnippet(code)
	res := result{Mode: "run", Notes: []string{}}
	if s.test {
		res.Mode = "test"
	}
	if !s.test && !s.hasMain && !s.wrapped {
		res.Phase, res.Program = "build", code
		res.Output = "This file has a package clause but no func main and no tests, so there is nothing to run."
		return res
	}
	bin := filepath.Join(dir, "prog")
	if runtime.GOOS == "windows" {
		bin += ".exe" // Windows only runs files with an executable extension
	}
	for attempt := 0; ; attempt++ {
		res.Program = s.source()
		if err := os.WriteFile(filepath.Join(dir, s.filename()), []byte(res.Program), 0o644); err != nil {
			return result{Phase: "build", Output: err.Error()}
		}
		args := []string{"build", "-o", bin, "."}
		if s.test {
			args = []string{"test", "-c", "-o", bin, "."}
		}
		out, err := goCmd(ctx, dir, args...)
		if err == nil {
			break
		}
		if attempt == 5 || !s.fix(res.Program, out, &res.Notes) {
			res.Phase, res.Output = "build", cleanBuildOutput(out)
			return res
		}
	}

	res.Phase = "run"
	args := []string{}
	if s.test {
		args = []string{"-test.v", "-test.bench=.", "-test.benchtime=100ms", "-test.benchmem"}
	}
	rctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	cmd := exec.CommandContext(rctx, bin, args...)
	cmd.Dir = dir
	cmd.WaitDelay = time.Second
	var buf capped
	cmd.Stdout, cmd.Stderr = &buf, &buf
	start := time.Now()
	err = cmd.Run()
	res.Ms = time.Since(start).Milliseconds()
	res.Output = buf.String()
	var ee *exec.ExitError
	switch {
	case rctx.Err() == context.DeadlineExceeded:
		res.TimedOut, res.Exit = true, -1
	case errors.As(err, &ee):
		res.Exit = ee.ExitCode()
	case err != nil:
		res.Exit, res.Output = -1, res.Output+err.Error()
	}
	return res
}

// goMod returns the snippet's go.mod. When the course root is known, it
// requires the course module so snippets can import gocourse/... packages.
func (e *executor) goMod() string {
	if e.root == "" {
		return "module play\n\ngo 1.24\n"
	}
	// Match the course's go directive: a lower one would ask for go mod tidy.
	goVersion := "1.24"
	if b, err := os.ReadFile(filepath.Join(e.root, "go.mod")); err == nil {
		if m := goDirective.FindSubmatch(b); m != nil {
			goVersion = string(m[1])
		}
	}
	return fmt.Sprintf("module play\n\ngo %s\n\nrequire gocourse v0.0.0\n\nreplace gocourse => %s\n", goVersion, strconv.Quote(e.root)) // quoted: paths may contain spaces or backslashes
}

var goDirective = regexp.MustCompile(`(?m)^go\s+(\S+)`)

// goCmd runs the go tool in dir and returns its combined output.
func goCmd(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var pkgHeader = regexp.MustCompile(`(?m)^# play\S*( \[\S+\])?\n`)

func cleanBuildOutput(out string) string {
	return strings.TrimSpace(pkgHeader.ReplaceAllString(out, ""))
}

// capped is a concurrency-safe buffer that drops output past maxOutput.
type capped struct {
	mu    sync.Mutex
	b     bytes.Buffer
	extra bool
}

func (c *capped) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if room := maxOutput - c.b.Len(); room < len(p) {
		c.b.Write(p[:max(room, 0)])
		c.extra = true
		return len(p), nil
	}
	return c.b.Write(p)
}

func (c *capped) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.extra {
		return c.b.String() + "\n… output truncated at 64 KiB"
	}
	return c.b.String()
}
