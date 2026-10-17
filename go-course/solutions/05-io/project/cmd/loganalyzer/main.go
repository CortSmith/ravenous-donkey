package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"gocourse/solutions/05-io/project/loganalyzer"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "loganalyzer:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("loganalyzer", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format (text or json)")
	top := fs.Int("top", 10, "number of top paths")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var input io.Reader = stdin
	if len(fs.Args()) > 0 {
		filename := fs.Args()[0]
		if filename != "-" {
			f, err := os.Open(filename)
			if err != nil {
				return fmt.Errorf("open %s: %w", filename, err)
			}
			defer f.Close()
			input = f
		}
	}

	a := loganalyzer.NewAnalyzer()
	sc := bufio.NewScanner(input)
	for sc.Scan() {
		if err := a.ProcessLine(sc.Text()); err != nil {
			fmt.Fprintf(stderr, "parse error: %v\n", err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	switch *format {
	case "text":
		return outputText(stdout, a, *top)
	case "json":
		return outputJSON(stdout, a, *top)
	default:
		return fmt.Errorf("unknown format: %s", *format)
	}
}

func outputText(w io.Writer, a *loganalyzer.Analyzer, top int) error {
	fmt.Fprintf(w, "Status Codes:\n")
	for _, sc := range a.StatusCounts() {
		fmt.Fprintf(w, "  %d: %d\n", sc.Status, sc.Count)
	}

	fmt.Fprintf(w, "\nTop Paths:\n")
	for _, pc := range a.TopPaths(top) {
		fmt.Fprintf(w, "  %s: %d\n", pc.Path, pc.Count)
	}

	lat := a.Latency()
	fmt.Fprintf(w, "\nLatency:\n")
	fmt.Fprintf(w, "  p50: %v\n", lat.P50)
	fmt.Fprintf(w, "  p95: %v\n", lat.P95)
	fmt.Fprintf(w, "  max: %v\n", lat.Max)

	return nil
}

func outputJSON(w io.Writer, a *loganalyzer.Analyzer, top int) error {
	out := jsonOutput{
		StatusCodes: []statusCodeJSON{},
		TopPaths:    []topPathJSON{},
	}

	for _, sc := range a.StatusCounts() {
		out.StatusCodes = append(out.StatusCodes, statusCodeJSON{Status: sc.Status, Count: sc.Count})
	}

	for _, pc := range a.TopPaths(top) {
		out.TopPaths = append(out.TopPaths, topPathJSON{Path: pc.Path, Count: pc.Count})
	}

	lat := a.Latency()
	out.Latency = latencyJSON{
		P50Ms: int64(lat.P50.Milliseconds()),
		P95Ms: int64(lat.P95.Milliseconds()),
		MaxMs: int64(lat.Max.Milliseconds()),
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

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
