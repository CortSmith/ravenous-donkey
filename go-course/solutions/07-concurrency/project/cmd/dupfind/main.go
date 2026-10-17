// Command dupfind reports files with identical contents under a directory.
//
// Usage:
//
//	go run ./solutions/07-concurrency/project/cmd/dupfind [-workers N] [-min-size BYTES] [dir]
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"gocourse/solutions/07-concurrency/project/dupfind"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("dupfind", flag.ContinueOnError)
	flags.SetOutput(stderr)
	workers := flags.Int("workers", 0, "max files hashed concurrently (0 = GOMAXPROCS)")
	minSize := flags.Int64("min-size", 1, "ignore files smaller than this many bytes")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderr, "usage: dupfind [-workers N] [-min-size BYTES] [dir]")
		return 2
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}

	groups, err := dupfind.Find(ctx, root, dupfind.Options{Workers: *workers, MinSize: *minSize})
	if err != nil {
		fmt.Fprintf(stderr, "dupfind: %v\n", err)
		return 1
	}
	if err := dupfind.WriteReport(stdout, groups); err != nil {
		fmt.Fprintf(stderr, "dupfind: %v\n", err)
		return 1
	}
	return 0
}
