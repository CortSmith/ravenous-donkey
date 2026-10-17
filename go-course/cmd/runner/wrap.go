package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"regexp"
	"slices"
	"strings"
)

// snippet is a lesson code block turned into a compilable file.
type snippet struct {
	wrapped bool     // false when the code already had a package clause
	test    bool     // build with go test instead of go build
	pkg     string   // package name for wrapped code
	imports []string // imports added automatically
	top     string   // package-level code
	body    string   // statements for func main
	hasMain bool
}

var (
	pkgClause = regexp.MustCompile(`(?m)^package\s+\w+`)
	mainFunc  = regexp.MustCompile(`(?m)^func\s+main\s*\(\s*\)`)
	testFunc  = regexp.MustCompile(`(?m)^func\s+(Test|Benchmark|Example|Fuzz)\w*\s*\(`)
	// A func declaration has a name, possibly after a receiver; "func() {"
	// at column 0 is a function literal and stays in main.
	funcDecl  = `func\s+[A-Za-z_]|func\s*\([^)]*\)\s*[A-Za-z_]\w*\s*[\[(]`
	topStart  = regexp.MustCompile(`^(` + funcDecl + `|type\b|import\b)`)
	declStart = regexp.MustCompile(`^(` + funcDecl + `|type\b|import\b|var\b|const\b)`)
)

// parseSnippet splits code into package-level declarations and main-body
// statements. Lines at column 0 that start a func, type or import go to
// package level, together with the comments directly above them; a
// declaration whose first line ends in "{" or "(" runs until the next line
// starting with "}" or ")". When the snippet declares its own funcs or
// types, column-0 var and const declarations go to package level too, so
// those funcs can use them. Everything else goes inside main.
func parseSnippet(code string) *snippet {
	code = strings.ReplaceAll(code, "\r\n", "\n")
	if pkgClause.MatchString(code) {
		s := &snippet{top: code, hasMain: mainFunc.MatchString(code)}
		s.test = !s.hasMain && testFunc.MatchString(code)
		return s
	}
	s := &snippet{wrapped: true, pkg: "main"}
	isTop := topStart
	if regexp.MustCompile(`(?m)^(type\b|` + funcDecl + `)`).MatchString(code) {
		isTop = declStart
	}
	var top, body, pending []string
	lines := strings.Split(strings.TrimRight(code, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(ln, "//") {
			pending = append(pending, ln)
			continue
		}
		if !isTop.MatchString(ln) {
			body = append(body, pending...)
			body = append(body, ln)
			pending = nil
			continue
		}
		top = append(top, pending...)
		pending = nil
		top = append(top, ln)
		if opensBlock(ln) {
			for i+1 < len(lines) {
				i++
				top = append(top, lines[i])
				if strings.HasPrefix(lines[i], "}") || strings.HasPrefix(lines[i], ")") {
					break
				}
			}
		}
		top = append(top, "")
	}
	body = append(body, pending...)
	s.top = strings.TrimSpace(strings.Join(top, "\n"))
	s.body = strings.Trim(strings.Join(body, "\n"), "\n")
	s.hasMain = mainFunc.MatchString(s.top)

	if s.hasMain && strings.TrimSpace(s.body) != "" {
		// A complete program without its package clause: keep the order.
		s.top, s.body = strings.TrimSpace(code), ""
	}
	if !s.hasMain && strings.TrimSpace(s.body) == "" && testFunc.MatchString(s.top) {
		s.test, s.pkg = true, "play"
	}
	return s
}

// opensBlock reports whether a declaration line leaves a { or ( open.
func opensBlock(ln string) bool {
	if i := strings.Index(ln, "//"); i >= 0 {
		ln = ln[:i]
	}
	ln = strings.TrimSpace(ln)
	return strings.HasSuffix(ln, "{") || strings.HasSuffix(ln, "(")
}

// filename is the file the snippet is compiled as.
func (s *snippet) filename() string {
	if s.test {
		return "play_test.go"
	}
	return "main.go"
}

// source renders the snippet as a gofmt-formatted Go file.
func (s *snippet) source() string {
	if !s.wrapped {
		return s.top
	}
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", s.pkg)
	if len(s.imports) > 0 {
		b.WriteString("import (\n")
		for _, p := range s.imports {
			fmt.Fprintf(&b, "\t%q\n", p)
		}
		b.WriteString(")\n\n")
	}
	if s.top != "" {
		b.WriteString(s.top + "\n\n")
	}
	if !s.hasMain && !s.test {
		b.WriteString("func main() {\n" + s.body + "\n}\n")
	}
	if out, err := format.Source([]byte(b.String())); err == nil {
		return string(out)
	}
	return b.String()
}

var (
	compileErr = regexp.MustCompile(`(?m)^\./\w+\.go:(\d+):\d+: (.*)$`)
	undefined  = regexp.MustCompile(`^undefined: (\w+)$`)
	unusedImp  = regexp.MustCompile(`^"([\w/]+)" imported and not used`)
	unusedVar  = regexp.MustCompile(`^declared and not used: (\w+)$`)
)

// fix applies the automatic repairs suggested by compiler output and
// reports what it changed. It returns false when nothing could be fixed.
// Only wrapped snippets are repaired; complete files are left as written.
func (s *snippet) fix(src, compilerOut string, notes *[]string) bool {
	if !s.wrapped {
		return false
	}
	hadImports := len(s.imports) > 0 // src starts with our import block
	changed := false
	unused := map[int][]string{}
	for _, m := range compileErr.FindAllStringSubmatch(compilerOut, -1) {
		msg := m[2]
		if u := undefined.FindStringSubmatch(msg); u != nil {
			p, ok := stdlib[u[1]]
			if ok && !slices.Contains(s.imports, p) &&
				regexp.MustCompile(`\b`+u[1]+`\.`).MatchString(s.top+"\n"+s.body) {
				s.imports = append(s.imports, p)
				*notes = append(*notes, fmt.Sprintf("added import %q", p))
				changed = true
			}
		} else if u := unusedImp.FindStringSubmatch(msg); u != nil {
			if i := slices.Index(s.imports, u[1]); i >= 0 {
				s.imports = slices.Delete(s.imports, i, i+1)
				changed = true
			}
		} else if u := unusedVar.FindStringSubmatch(msg); u != nil {
			var line int
			fmt.Sscan(m[1], &line)
			unused[line] = append(unused[line], u[1])
		}
	}
	if len(unused) > 0 {
		if top, body, ok := s.silence(src, hadImports, unused); ok {
			s.top, s.body = top, body
			for _, names := range unused {
				for _, n := range names {
					*notes = append(*notes, fmt.Sprintf("added `_ = %s` (unused variable)", n))
				}
			}
			changed = true
		}
	}
	return changed
}

// silence inserts "_ = name" after the statement that declares each unused
// variable (or at the top of the block for a for/if/switch header), then
// splits the patched file back into top and body. hadImports reports
// whether src begins with an import block that source generated.
func (s *snippet) silence(src string, hadImports bool, unused map[int][]string) (top, body string, ok bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return "", "", false
	}
	after := map[int][]string{} // insert after this line → names
	for line, names := range unused {
		var best ast.Stmt
		ast.Inspect(f, func(n ast.Node) bool {
			b, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for _, st := range b.List {
				if fset.Position(st.Pos()).Line <= line && line <= fset.Position(st.End()).Line {
					best = st // inner blocks are visited later, so the innermost wins
				}
			}
			return true
		})
		if best == nil {
			return "", "", false
		}
		at := fset.Position(best.End()).Line
		if lb := headerBrace(best); lb.IsValid() && line <= fset.Position(lb).Line {
			at = fset.Position(lb).Line
		}
		after[at] = append(after[at], names...)
	}
	lines := strings.Split(src, "\n")
	var out []string
	for i, ln := range lines {
		out = append(out, ln)
		for _, n := range after[i+1] {
			out = append(out, "_ = "+n)
		}
	}
	patched := strings.Join(out, "\n")

	// Re-split: everything inside func main is the body, the rest is top.
	f, err = parser.ParseFile(fset, "", patched, parser.ParseComments)
	if err != nil {
		return "", "", false
	}
	var mainFn *ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "main" && fd.Recv == nil {
			mainFn = fd
		}
	}
	// Drop the package clause and the import block we generated.
	start := fset.Position(f.Name.End()).Offset
	if hadImports {
		start = fset.Position(f.Decls[0].End()).Offset
	}
	if mainFn == nil || s.hasMain {
		return strings.TrimSpace(patched[start:]), s.body, true
	}
	top = strings.TrimSpace(patched[start:fset.Position(mainFn.Pos()).Offset])
	body = patched[fset.Position(mainFn.Body.Lbrace).Offset+1 : fset.Position(mainFn.Body.Rbrace).Offset]
	return top, strings.Trim(body, "\n"), true
}

// headerBrace returns the opening brace of a statement whose header can
// declare variables (for, range, if, switch), or token.NoPos.
func headerBrace(st ast.Stmt) token.Pos {
	switch st := st.(type) {
	case *ast.ForStmt:
		return st.Body.Lbrace
	case *ast.RangeStmt:
		return st.Body.Lbrace
	case *ast.IfStmt:
		return st.Body.Lbrace
	case *ast.SwitchStmt:
		return st.Body.Lbrace
	}
	return token.NoPos
}

// stdlib maps package names used in lesson snippets to import paths.
var stdlib = map[string]string{
	"atomic": "sync/atomic", "base64": "encoding/base64", "big": "math/big",
	"binary": "encoding/binary", "bits": "math/bits", "bufio": "bufio",
	"bytes": "bytes", "cmp": "cmp", "context": "context", "crc32": "hash/crc32",
	"csv": "encoding/csv", "debug": "runtime/debug", "errors": "errors",
	"exec": "os/exec", "filepath": "path/filepath", "flag": "flag",
	"fmt": "fmt", "fnv": "hash/fnv", "fs": "io/fs", "fstest": "testing/fstest",
	"gzip": "compress/gzip", "heap": "container/heap", "hex": "encoding/hex",
	"http": "net/http", "httptest": "net/http/httptest", "io": "io",
	"iotest": "testing/iotest", "iter": "iter", "json": "encoding/json",
	"list": "container/list", "log": "log", "maphash": "hash/maphash",
	"maps": "maps", "math": "math", "mime": "mime", "net": "net", "netip": "net/netip",
	"httputil": "net/http/httputil", "cookiejar": "net/http/cookiejar", "textproto": "net/textproto",
	"pem": "encoding/pem", "sha1": "crypto/sha1", "md5": "crypto/md5", "hmac": "crypto/hmac",
	"utf16": "unicode/utf16", "ring": "container/ring", "zip": "archive/zip", "tar": "archive/tar",
	"os": "os", "path": "path", "pprof": "runtime/pprof", "rand": "math/rand/v2",
	"reflect": "reflect", "regexp": "regexp", "runtime": "runtime",
	"sha256": "crypto/sha256", "signal": "os/signal", "slices": "slices",
	"slog": "log/slog", "sort": "sort", "strconv": "strconv",
	"strings": "strings", "sync": "sync", "syscall": "syscall", "tabwriter": "text/tabwriter",
	"template": "text/template", "testing": "testing", "time": "time",
	"trace": "runtime/trace", "unicode": "unicode", "unique": "unique",
	"url": "net/url", "utf8": "unicode/utf8", "weak": "weak", "xml": "encoding/xml",
}
