package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A block is one ```go code block from a Markdown file.
type block struct {
	file   string
	line   int    // line of the opening fence
	expect string // "", "compile-error" or "runtime-error"
	code   string
}

var (
	fenceOpen  = regexp.MustCompile("^(\\s*)```go(?:\\s+(.*))?$")
	fenceClose = regexp.MustCompile("^\\s*```\\s*$")
	expectAttr = regexp.MustCompile(`\bexpect=(\S+)`)
)

// expectations lists the valid values of a block's expect= attribute.
var expectations = map[string]bool{"": true, "compile-error": true, "runtime-error": true}

// extractBlocks returns the ```go blocks in a Markdown document. A fence
// indented inside a list item has that indentation removed from its lines.
func extractBlocks(file, text string) []block {
	var out []block
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		m := fenceOpen.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		b := block{file: file, line: i + 1}
		if e := expectAttr.FindStringSubmatch(m[2]); e != nil {
			b.expect = e[1]
		}
		var code []string
		for i++; i < len(lines) && !fenceClose.MatchString(lines[i]); i++ {
			code = append(code, strings.TrimPrefix(lines[i], m[1]))
		}
		b.code = strings.Join(code, "\n") + "\n"
		out = append(out, b)
	}
	return out
}

// check runs every ```go block in the Markdown files under paths and
// reports the blocks whose outcome does not match their expect= attribute
// (by default: compiles, exits 0, finishes in time). It returns the number
// of failing blocks.
//
// With strict set, a block also fails when it only compiles because the
// runner silenced an unused variable: the code on the page has a compile error.
func check(ctx context.Context, ex *executor, paths []string, verbose, strict bool, w io.Writer) (int, error) {
	var blocks []block
	for _, p := range paths {
		err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != p {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			name := path
			if rel, err := filepath.Rel(ex.root, path); err == nil && !strings.HasPrefix(rel, "..") {
				name = rel
			}
			blocks = append(blocks, extractBlocks(name, string(b))...)
			return nil
		})
		if err != nil {
			return 0, err
		}
	}

	failed := 0
	for _, b := range blocks {
		r := ex.Run(ctx, b.code)
		got := outcome(r)
		if strict && got == "ok" {
			for _, n := range r.Notes {
				if strings.HasPrefix(n, "added `_ = ") {
					got = "ok only after " + n
					break
				}
			}
		}
		if !expectations[b.expect] {
			got = "invalid expect=" + b.expect
		}
		want := b.expect
		if want == "" {
			want = "ok"
		}
		if got == want {
			if verbose {
				fmt.Fprintf(w, "ok   %s:%d\n", b.file, b.line)
				for _, ln := range strings.Split(strings.TrimRight(r.Output, "\n"), "\n") {
					fmt.Fprintf(w, "     | %s\n", ln)
				}
			}
			continue
		}
		failed++
		fmt.Fprintf(w, "FAIL %s:%d: want %s, got %s\n", b.file, b.line, want, got)
		out := strings.Split(strings.TrimSpace(r.Output), "\n")
		for _, ln := range out[:min(6, len(out))] {
			fmt.Fprintf(w, "     | %s\n", ln)
		}
	}
	fmt.Fprintf(w, "%d of %d Go blocks behave as expected\n", len(blocks)-failed, len(blocks))
	return failed, nil
}

// outcome classifies a run as "ok", "compile-error", "runtime-error" or "timeout".
func outcome(r result) string {
	switch {
	case r.Phase == "build":
		return "compile-error"
	case r.TimedOut:
		return "timeout"
	case r.Exit != 0:
		return "runtime-error"
	}
	return "ok"
}
