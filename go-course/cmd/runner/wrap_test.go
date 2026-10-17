package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSnippet(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		wantTop         string // substring of top ("" = top must be empty)
		wantBody        string // substring of body
		wrapped, test   bool
		wantMainPresent bool
	}{
		{name: "statements", code: "x := 1\nfmt.Println(x)\n", wantBody: "fmt.Println(x)", wrapped: true},
		{name: "func and call", code: "func double(n int) int {\n\treturn n * 2\n}\n\nfmt.Println(double(2))\n",
			wantTop: "func double", wantBody: "fmt.Println(double(2))", wrapped: true},
		{name: "doc comment moves with func", code: "// double doubles.\nfunc double(n int) int { return n * 2 }\n",
			wantTop: "// double doubles.", wrapped: true},
		{name: "var goes to top with funcs", code: "var ErrX = errors.New(\"x\")\nfunc f() error { return ErrX }\n",
			wantTop: "var ErrX", wrapped: true},
		{name: "var stays in main without funcs", code: "var n int\nfmt.Println(n)\n", wantBody: "var n int", wrapped: true},
		{name: "complete program without package", code: "func main() {\n\tfmt.Println(1)\n}\n",
			wantTop: "func main", wrapped: true, wantMainPresent: true},
		{name: "test funcs", code: "func TestX(t *testing.T) {\n\tt.Log(1)\n}\n", wantTop: "func TestX", wrapped: true, test: true},
		{name: "func literal stays in main", code: "func() {\n\tfmt.Println(1)\n}()\n", wantBody: "func() {", wrapped: true},
		{name: "method goes to top", code: "type T int\n\nfunc (t T) M() {}\n\nT(1).M()\n", wantTop: "func (t T) M()", wantBody: "T(1).M()", wrapped: true},
		{name: "full file", code: "package main\n\nfunc main() {}\n", wantTop: "package main", wantMainPresent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := parseSnippet(tt.code)
			if s.wrapped != tt.wrapped || s.test != tt.test || s.hasMain != tt.wantMainPresent {
				t.Errorf("wrapped, test, hasMain = %v, %v, %v; want %v, %v, %v",
					s.wrapped, s.test, s.hasMain, tt.wrapped, tt.test, tt.wantMainPresent)
			}
			if tt.wantTop == "" && s.top != "" || !strings.Contains(s.top, tt.wantTop) {
				t.Errorf("top = %q, want it to contain %q", s.top, tt.wantTop)
			}
			if !strings.Contains(s.body, tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", s.body, tt.wantBody)
			}
		})
	}
}

func TestRun(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not in PATH")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	ex := &executor{root: root, timeout: 5 * time.Second, slots: make(chan struct{}, 1)}
	tests := []struct {
		name, code, wantOut, wantPhase string
		wantNote                       string
	}{
		{"adds imports", "fmt.Println(strings.ToUpper(\"go\"))", "GO", "run", `added import "strings"`},
		{"silences unused", "x := 1\nfmt.Println(\"ok\")", "ok", "run", "_ = x"},
		{"unused in range header", "for i, v := range []int{1} {\n\tfmt.Println(i)\n}", "0", "run", "_ = v"},
		{"panic output", "var m map[string]int\nm[\"x\"] = 1", "assignment to entry in nil map", "run", ""},
		{"compile error", "fmt.Println(undefinedThing)", "undefined: undefinedThing", "build", ""},
		{"tests", "func TestOK(t *testing.T) {}", "--- PASS: TestOK", "run", ""},
		{"external test package has no header", "package play_test\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) { undefinedThing() }", "./play_test.go", "build", ""},
		{"test build error has no package header", "func TestX(t *testing.T) { undefinedThing() }", "./play_test.go", "build", ""},
		{"imports course packages", "package main\n\nimport (\n\t\"fmt\"\n\n\t\"gocourse/solutions/04-testing/project/semver\"\n)\n\nfunc main() {\n\tv, err := semver.Parse(\"1.2.3\")\n\tfmt.Println(v, err)\n}\n", "1.2.3", "run", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ex.Run(context.Background(), tt.code)
			if r.Phase != tt.wantPhase || !strings.Contains(r.Output, tt.wantOut) {
				t.Fatalf("Run(%q) = phase %q, output %q; want phase %q, output containing %q\nprogram:\n%s",
					tt.code, r.Phase, r.Output, tt.wantPhase, tt.wantOut, r.Program)
			}
			if strings.HasPrefix(r.Output, "#") {
				t.Errorf("output starts with a package header: %q", r.Output)
			}
			if tt.wantNote != "" && !strings.Contains(strings.Join(r.Notes, "\n"), tt.wantNote) {
				t.Errorf("notes = %q, want one containing %q", r.Notes, tt.wantNote)
			}
		})
	}
}

func TestRequestGuards(t *testing.T) {
	h := newHandler(t.TempDir(), &executor{timeout: time.Second, slots: make(chan struct{}, 1)})
	tests := []struct {
		name, host, origin, ctype string
		want                      int
	}{
		{"other site", "localhost:8090", "https://evil.example", "application/json", http.StatusForbidden},
		{"no origin", "localhost:8090", "", "application/json", http.StatusForbidden},
		{"simple content type", "localhost:8090", "http://localhost:8090", "text/plain", http.StatusForbidden},
		{"rebound host", "evil.example:8090", "http://evil.example:8090", "application/json", http.StatusForbidden},
		{"same origin, bad body", "localhost:8090", "http://localhost:8090", "application/json", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/run", strings.NewReader("not json"))
			req.Host = tt.host
			req.Header.Set("Content-Type", tt.ctype)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestExtractBlocks(t *testing.T) {
	md := "intro\n```go\nfmt.Println(1)\n```\n\n- item\n  ```go expect=compile-error\n  x := \n  ```\n```sh\nls\n```\n"
	got := extractBlocks("f.md", md)
	if len(got) != 2 {
		t.Fatalf("extractBlocks found %d blocks, want 2: %+v", len(got), got)
	}
	if got[0].line != 2 || got[0].expect != "" || got[0].code != "fmt.Println(1)\n" {
		t.Errorf("block 0 = %+v", got[0])
	}
	if got[1].line != 7 || got[1].expect != "compile-error" || got[1].code != "x := \n" {
		t.Errorf("block 1 = %+v (the list indentation should be removed)", got[1])
	}
}
