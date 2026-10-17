// Command kvd serves the durable key-value store over HTTP.
//
//	kvd -addr 127.0.0.1:8080 -db kvd.log
//
// It shuts down gracefully on SIGINT or SIGTERM: it stops accepting
// connections, waits for in-flight requests, stops the janitor, and closes
// the database log.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "kvd:", err)
		stop() // os.Exit skips deferred calls
		os.Exit(1)
	}
}

// run is main without the process-global parts (signals, os.Exit), so it can
// be tested: canceling ctx must trigger the same graceful shutdown as a
// signal. Write logs (slog) to stdout and flag errors to stderr.
//
// The tests start run with -addr 127.0.0.1:0 (a random free port) and find
// the real address in the "server started" log line, which must contain
// addr=<host:port> (log ln.Addr(), not the flag value).
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	// TODO: parse -addr and -db with a flag.NewFlagSet (ContinueOnError)
	// TODO: open the database with store.Open; close it on EVERY return path
	// TODO: net.Listen, then start the janitor in a goroutine
	// TODO: create an http.Server (set ReadHeaderTimeout) with Server.Register
	// TODO: Serve in a goroutine; return its error if it fails before ctx is done
	// TODO: on ctx.Done: Shutdown with a 5s timeout, stop the janitor, close the db
	_, _, _, _ = ctx, args, stdout, stderr
	return errors.New("not implemented")
}
