// Command runner serves the course site (index.html) on localhost and runs
// the Go snippets from the lessons when you press Run.
//
// Start it from the course root, then open the URL it prints:
//
//	go run ./cmd/runner
//
// With -check, it runs every ```go block in the given Markdown files or
// directories instead (default: the whole course) and reports the blocks
// that fail. A block that is meant to fail says so in its fence:
// ```go expect=compile-error or ```go expect=runtime-error.
//
//	go run ./cmd/runner -check modules/01-basics
//
// Lesson snippets are usually fragments, so the runner wraps them before
// compiling: top-level funcs and types stay at package level, every other
// line goes inside func main, missing standard-library imports are added,
// and unused variables are silenced with "_ = x". The page shows the exact
// program that ran.
//
// The server executes arbitrary code, so it only accepts requests that come
// from its own page on a loopback address.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("runner", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8090", "listen address")
	root := fs.String("root", ".", "course root (the directory containing index.html)")
	timeout := fs.Duration("timeout", 10*time.Second, "time limit for each snippet run")
	checkMode := fs.Bool("check", false, "run the Go blocks in the Markdown files given as arguments and report failures")
	strict := fs.Bool("strict", false, "with -check, also fail blocks that only compile after an unused variable is silenced")
	verbose := fs.Bool("v", false, "with -check, also list the blocks that pass, with their output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, "runner:", err)
		return 1
	}
	if _, err := os.Stat(filepath.Join(abs, "index.html")); err != nil {
		fmt.Fprintf(stderr, "runner: no index.html in %s (run from the course root or pass -root)\n", abs)
		return 1
	}

	if *checkMode {
		paths := fs.Args()
		if len(paths) == 0 {
			paths = []string{abs}
		}
		ex := &executor{root: abs, timeout: *timeout, slots: make(chan struct{}, 1)}
		failed, err := check(context.Background(), ex, paths, *verbose, *strict, stdout)
		if err != nil {
			fmt.Fprintln(stderr, "runner:", err)
			return 1
		}
		if failed > 0 {
			return 1
		}
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           newHandler(abs, &executor{root: abs, timeout: *timeout, slots: make(chan struct{}, 2)}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(stderr, "runner:", err)
		return 1
	}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	fmt.Fprintf(stdout, "Go course: http://localhost:%s/  (Ctrl+C to stop)\n", port)

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		fmt.Fprintln(stderr, "runner:", err)
		return 1
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(stderr, "runner:", err)
		return 1
	}
	return 0
}

func newHandler(root string, ex *executor) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.Dir(root)))
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"go": runtime.Version()})
	})
	mux.HandleFunc("POST /api/run", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, ex.Run(r.Context(), req.Code))
	})
	return loopbackOnly(mux)
}

// loopbackOnly rejects requests whose Host header is not a loopback name.
// That blocks DNS-rebinding attacks, where a hostile site points its own
// domain at 127.0.0.1 to reach this server.
func loopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		switch strings.Trim(host, "[]") {
		case "localhost", "127.0.0.1", "::1":
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "forbidden host", http.StatusForbidden)
		}
	})
}

// sameOrigin reports whether r was sent by a page served from this server.
// Browsers always send Origin on cross-origin POSTs, and a JSON
// Content-Type forces a CORS preflight that this server never approves.
func sameOrigin(r *http.Request) bool {
	return r.Header.Get("Origin") == "http://"+r.Host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
