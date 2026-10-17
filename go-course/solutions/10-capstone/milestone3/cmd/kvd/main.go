// Command kvd serves the durable key-value store over HTTP.
//
//	kvd -addr 127.0.0.1:8080 -db kvd.log
//
// It shuts down gracefully on SIGINT or SIGTERM: it stops accepting
// connections, waits up to 5 seconds for in-flight requests, stops the
// janitor, and closes the database log.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gocourse/solutions/10-capstone/milestone3/store"
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
// be tested: cancel ctx to trigger the same graceful shutdown as a signal.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) (err error) {
	fs := flag.NewFlagSet("kvd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address")
	dbPath := fs.String("db", "kvd.log", "database log path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	db, err := store.Open(*dbPath, store.Options{})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	// Close the log on every return path, and report its error: a failed
	// Close can mean buffered writes never reached the disk.
	defer func() {
		if cerr := db.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close database: %w", cerr))
		}
	}()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	janitorCtx, stopJanitor := context.WithCancel(ctx)
	janitorDone := make(chan struct{})
	go func() {
		defer close(janitorDone)
		db.RunJanitor(janitorCtx, time.Minute)
	}()
	defer func() {
		stopJanitor()
		<-janitorDone // don't close the database under a running janitor
	}()

	mux := http.NewServeMux()
	store.NewServer(db, logger).Register(mux)
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // slow-header (Slowloris) protection
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       time.Minute,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	logger.Info("server started", "addr", ln.Addr().String())

	select {
	case err := <-serveErr:
		// Serve returned without Shutdown being called: a real failure.
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	logger.Info("shutdown complete")
	return nil
}
