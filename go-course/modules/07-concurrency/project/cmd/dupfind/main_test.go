package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"a": "same", "b": "same", "c": "diff!"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // substring
		wantStderr string // substring
	}{
		{"finds duplicates", []string{dir}, 0, "1 groups, 4 bytes reclaimable", ""},
		{"lists both paths", []string{"-workers", "2", dir}, 0, filepath.Join(dir, "b"), ""},
		{"min-size excludes", []string{"-min-size", "5", dir}, 0, "0 groups, 0 bytes reclaimable", ""},
		{"bad flag", []string{"-nope"}, 2, "", "flag provided but not defined"},
		{"too many args", []string{dir, dir}, 2, "", "usage"},
		{"missing dir", []string{filepath.Join(dir, "missing")}, 1, "", "dupfind:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(t.Context(), tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("run(%q) exit code = %d, want %d; stderr: %s", tt.args, code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("run(%q) stdout = %q, want it to contain %q", tt.args, stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("run(%q) stderr = %q, want it to contain %q", tt.args, stderr.String(), tt.wantStderr)
			}
		})
	}
}
