package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "loganalyzer:", err)
		os.Exit(1)
	}
}

// TODO: implement run to:
// 1. Parse flags: -format (text|json, default text) and -top (default 10)
// 2. Get the filename from remaining args, defaulting to "-" (stdin)
// 3. Open the file (or use stdin if "-")
// 4. Stream the logs through an Analyzer using bufio.Scanner
// 5. Output the results in the requested format
// 6. Return an error if parsing fails
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	_ = args
	_ = stdin
	_ = stdout
	_ = stderr
	return nil
}

// TODO: implement outputText and outputJSON to format and write the results.

// These helper types are for JSON marshaling
type jsonOutput struct {
	StatusCodes []statusCodeJSON `json:"status_codes"`
	TopPaths    []topPathJSON    `json:"top_paths"`
	Latency     latencyJSON      `json:"latency"`
}

type statusCodeJSON struct {
	Status int `json:"status"`
	Count  int `json:"count"`
}

type topPathJSON struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

type latencyJSON struct {
	P50Ms int64 `json:"p50_ms"`
	P95Ms int64 `json:"p95_ms"`
	MaxMs int64 `json:"max_ms"`
}
