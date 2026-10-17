// Command dupfind reports files with identical contents under a directory.
//
// Usage:
//
//	go run ./modules/07-concurrency/project/cmd/dupfind [-workers N] [-min-size BYTES] [dir]
package main

import (
	"context"
	"io"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	// TODO: implement
	return 2
}
