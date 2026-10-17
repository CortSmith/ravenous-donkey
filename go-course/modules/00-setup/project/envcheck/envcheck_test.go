package envcheck

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

// fixedEnv makes the -v output identical on every machine.
var fixedEnv = Env{GoVersion: "go1.24.2", OS: "linux", Arch: "amd64", CPUs: 8, InContainer: true}

func TestRun(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  Env
		want string
	}{
		{"no flags", nil, fixedEnv, "Hello, World!\nYour Go environment works.\n"},
		{"name", []string{"-name", "Ada"}, fixedEnv, "Hello, Ada!\nYour Go environment works.\n"},
		{"name with equals", []string{"-name=Ada"}, fixedEnv, "Hello, Ada!\nYour Go environment works.\n"},
		{"blank name", []string{"-name", "  "}, fixedEnv, "Hello, World!\nYour Go environment works.\n"},
		{"verbose in container", []string{"-v", "-name", "Ada"}, fixedEnv,
			"Hello, Ada!\nYour Go environment works.\n\ngo version: go1.24.2\nplatform: linux/amd64\ncpus: 8\ncontainer: yes\n"},
		{"verbose on host", []string{"-v"}, Env{GoVersion: "go1.27.0", OS: "darwin", Arch: "arm64", CPUs: 10},
			"Hello, World!\nYour Go environment works.\n\ngo version: go1.27.0\nplatform: darwin/arm64\ncpus: 10\ncontainer: no\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, tt.env, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("Run(%q) exit code = %d, want 0 (stderr: %q)", tt.args, code, stderr.String())
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run(%q) stdout = %q, want %q", tt.args, got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Errorf("Run(%q) stderr = %q, want empty", tt.args, stderr.String())
			}
		})
	}
}

func TestRunUsageErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string // a substring the error message must contain
	}{
		{"unknown flag", []string{"-x"}, "flag provided but not defined: -x"},
		{"positional argument", []string{"Ada"}, `unexpected argument "Ada"`},
		{"argument after flags", []string{"-v", "extra"}, `unexpected argument "extra"`},
		{"missing flag value", []string{"-name"}, "flag needs an argument"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(tt.args, fixedEnv, &stdout, &stderr); code != 2 {
				t.Errorf("Run(%q) exit code = %d, want 2", tt.args, code)
			}
			if stdout.Len() != 0 {
				t.Errorf("Run(%q) wrote to stdout on a usage error: %q", tt.args, stdout.String())
			}
			errOut := stderr.String()
			if !strings.Contains(errOut, tt.wantStderr) {
				t.Errorf("Run(%q) stderr = %q, want it to contain %q", tt.args, errOut, tt.wantStderr)
			}
			if !strings.Contains(errOut, "Usage of envcheck") {
				t.Errorf("Run(%q) stderr does not show the usage: %q", tt.args, errOut)
			}
		})
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-h"}, fixedEnv, &stdout, &stderr); code != 0 {
		t.Errorf("Run(-h) exit code = %d, want 0", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("Run(-h) stdout = %q, want empty (usage goes to stderr)", stdout.String())
	}
	for _, want := range []string{"-name", "-v"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("Run(-h) usage does not mention %s: %q", want, stderr.String())
		}
	}
}

func TestDetectEnv(t *testing.T) {
	env := DetectEnv()
	if env.GoVersion != runtime.Version() || env.OS != runtime.GOOS || env.Arch != runtime.GOARCH {
		t.Errorf("DetectEnv() = %+v, want GoVersion=%s OS=%s Arch=%s", env, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	}
	if env.CPUs < 1 {
		t.Errorf("DetectEnv().CPUs = %d, want >= 1", env.CPUs)
	}
	t.Logf("detected: %+v", env) // visible with go test -v
}
