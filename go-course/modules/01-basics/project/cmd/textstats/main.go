// Command textstats prints statistics about text read from files or stdin.
//
// Usage:
//
//	textstats [-n N] [-all] [file ...]
//
// With no file arguments it reads standard input.
package main

import (
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is the testable body of main. It returns the process exit code:
// 0 on success, 1 if an input cannot be read, 2 for invalid usage.
//
// Flags:
//
//	-n int   number of most frequent words to show (default 10; negative is a usage error)
//	-all     include stopwords in the top words (default: use textstats.DefaultStopwords)
//
// Remaining arguments are file names whose contents are joined with "\n";
// with none, all of stdin is read. On success the report (Report.String) is
// written to stdout. Errors go to stderr only, prefixed with "textstats: ".
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// TODO: use flag.NewFlagSet("textstats", flag.ContinueOnError) with
	// SetOutput(stderr), read the inputs, then call textstats.Analyze from
	// gocourse/modules/01-basics/project/textstats.
	return 1
}
