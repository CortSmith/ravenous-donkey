// Command textstats prints statistics about text read from files or stdin.
//
// Usage:
//
//	textstats [-n N] [-all] [file ...]
//
// With no file arguments it reads standard input.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gocourse/solutions/01-basics/project/textstats"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is the testable body of main. It returns the process exit code:
// 0 on success, 1 if an input cannot be read, 2 for invalid usage.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("textstats", flag.ContinueOnError)
	fs.SetOutput(stderr)
	n := fs.Int("n", 10, "number of most frequent words to show")
	all := fs.Bool("all", false, "include stopwords in the top words")
	if err := fs.Parse(args); err != nil {
		return 2 // flag already printed the error and usage
	}
	if *n < 0 {
		fmt.Fprintln(stderr, "textstats: -n must not be negative")
		return 2
	}

	text, err := readInputs(fs.Args(), stdin)
	if err != nil {
		fmt.Fprintf(stderr, "textstats: %v\n", err)
		return 1
	}

	opts := textstats.Options{TopN: *n, Stopwords: textstats.DefaultStopwords}
	if *all {
		opts.Stopwords = nil
	}
	fmt.Fprint(stdout, textstats.Analyze(text, opts))
	return 0
}

// readInputs returns the contents of the named files joined by newlines, or
// all of stdin when no names are given.
func readInputs(names []string, stdin io.Reader) (string, error) {
	if len(names) == 0 {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(b), nil
	}
	var sb strings.Builder
	for _, name := range names {
		b, err := os.ReadFile(name)
		if err != nil {
			return "", err // *PathError already names the file
		}
		sb.Write(b)
		sb.WriteByte('\n') // so the last word of one file can't join the next
	}
	return sb.String(), nil
}
