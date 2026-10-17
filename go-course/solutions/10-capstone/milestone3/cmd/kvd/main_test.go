package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// startKVD runs run() in a goroutine on a random port and returns the base
// URL, a function that triggers graceful shutdown (as SIGTERM would) and
// returns run's error, and the captured log output.
func startKVD(t *testing.T, dbPath string) (baseURL string, shutdown func() error, logs *bytes.Buffer) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	pr, pw := io.Pipe()
	logs = new(bytes.Buffer)
	done := make(chan error, 1)
	go func() {
		err := run(ctx, []string{"-addr", "127.0.0.1:0", "-db", dbPath}, pw, io.Discard)
		pw.Close()
		done <- err
	}()

	// The first log line announces the address; keep copying the rest.
	addrRE := regexp.MustCompile(`addr=(\S+)`)
	lines := bufio.NewScanner(pr)
	found := make(chan string, 1)
	scanned := make(chan struct{}) // closed when all output has been copied to logs
	go func() {
		defer close(scanned)
		for lines.Scan() {
			line := lines.Text()
			logs.WriteString(line + "\n")
			if strings.Contains(line, "server started") {
				if m := addrRE.FindStringSubmatch(line); m != nil {
					found <- m[1]
				}
			}
		}
		close(found)
		io.Copy(io.Discard, pr) // never block run's logger if Scan stopped early
	}()

	select {
	case addr, ok := <-found:
		if !ok {
			cancel()
			t.Fatalf("run exited before logging \"server started\": %v", <-done)
		}
		baseURL = "http://" + addr
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("timed out waiting for \"server started\" log line")
	}

	shutdown = func() error {
		cancel()
		select {
		case err := <-done:
			<-scanned // run closed the pipe; wait until logs is complete
			return err
		case <-time.After(10 * time.Second):
			t.Fatal("run did not return within 10s of cancellation")
			return nil
		}
	}
	return baseURL, shutdown, logs
}

func do(t *testing.T, method, url, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// TestRunServesAndPersistsAcrossRestart checks the whole program: it serves
// the API, shuts down cleanly when its context is canceled, and data written
// before shutdown is still there after a restart on the same log file.
func TestRunServesAndPersistsAcrossRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kvd.log")

	base, shutdown, logs := startKVD(t, dbPath)
	if code, body := do(t, http.MethodPut, base+"/kv/greeting", `{"value":"hello"}`); code != http.StatusOK {
		t.Fatalf("PUT /kv/greeting = %d %s, want 200", code, body)
	}
	if err := shutdown(); err != nil {
		t.Fatalf("run() after shutdown = %v, want nil", err)
	}
	for _, msg := range []string{"shutting down", "shutdown complete"} {
		if !strings.Contains(logs.String(), msg) {
			t.Errorf("logs do not contain %q:\n%s", msg, logs)
		}
	}

	base, shutdown, _ = startKVD(t, dbPath)
	defer shutdown()
	code, body := do(t, http.MethodGet, base+"/kv/greeting", "")
	if code != http.StatusOK || !strings.Contains(body, `"hello"`) {
		t.Errorf("GET /kv/greeting after restart = %d %s, want 200 with \"hello\"", code, body)
	}
}

func TestRunRejectsBadFlags(t *testing.T) {
	err := run(context.Background(), []string{"-nope"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("run(-nope) = nil, want a flag error")
	}
}

func TestRunReportsListenError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kvd.log")
	err := run(context.Background(), []string{"-addr", "256.0.0.1:0", "-db", dbPath}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("run(bad addr) = %v, want a listen error", err)
	}
}
