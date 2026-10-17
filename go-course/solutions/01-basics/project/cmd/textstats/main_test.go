package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	in := strings.NewReader("The fox and the dog. The fox!")
	code := run([]string{"-n", "2"}, in, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	want := "words: 7\n" +
		"unique: 4\n" +
		"longest: and\n" +
		"average length: 3.00\n" +
		"top words:\n" +
		"  1. fox (2)\n" +
		"  2. dog (1)\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout =\n%s\nwant\n%s", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunAllFlagKeepsStopwords(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-all", "-n", "1"}, strings.NewReader("the the fox"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "  1. the (2)\n") {
		t.Errorf("with -all, stdout =\n%s\nwant it to list \"the (2)\" first", stdout.String())
	}
}

func TestRunDefaultTopIsTen(t *testing.T) {
	var stdout, stderr bytes.Buffer
	text := "a1 a2 a3 a4 a5 a6 a7 a8 a9 b1 b2 b3"
	if code := run(nil, strings.NewReader(text), &stdout, &stderr); code != 0 {
		t.Fatalf("run exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	if got := strings.Count(stdout.String(), ". "); got != 10 {
		t.Errorf("default output lists %d top words, want 10:\n%s", got, stdout.String())
	}
}

func TestRunFiles(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	// a.txt has no trailing newline: "end" and "start" must stay separate words.
	if err := os.WriteFile(a, []byte("gopher end"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("start gopher"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"-n", "1", a, b}, strings.NewReader("ignored stdin"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"words: 4\n", "unique: 3\n", "  1. gopher (2)\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout =\n%s\nwant it to contain %q", out, want)
		}
	}
}

func TestRunErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.txt")
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantErr  string // substring expected on stderr
	}{
		{"unknown flag", []string{"-bogus"}, 2, "bogus"},
		{"bad -n value", []string{"-n", "many"}, 2, "invalid value"},
		{"negative -n", []string{"-n", "-1"}, 2, "-n"},
		{"missing file", []string{missing}, 1, "does-not-exist.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("run(%q) exit code = %d, want %d", tt.args, code, tt.wantCode)
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Errorf("run(%q) stderr = %q, want it to mention %q", tt.args, stderr.String(), tt.wantErr)
			}
			if stdout.Len() != 0 {
				t.Errorf("run(%q) wrote %q to stdout on error, want nothing", tt.args, stdout.String())
			}
		})
	}
}
