package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// openTestLog opens a log at a fresh temp path and registers Close.
func openTestLog(t *testing.T, path string) *Log {
	t.Helper()
	l, err := OpenLog(path, false)
	if err != nil {
		t.Fatalf("OpenLog(%q) error = %v", path, err)
	}
	if l == nil {
		t.Fatalf("OpenLog(%q) returned nil *Log and nil error", path)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

// replayAll replays l and returns every record.
func replayAll(t *testing.T, l *Log) []Record {
	t.Helper()
	var got []Record
	if err := l.Replay(func(r Record) error {
		got = append(got, r)
		return nil
	}); err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	return got
}

// writeFile replaces path's contents.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readFile returns path's contents.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var tricky = []Record{
	{Op: OpSet, Key: "a", Value: "1"},
	{Op: OpSet, Key: "newline", Value: "line1\nline2\n"},
	{Op: OpSet, Key: "quotes", Value: `he said "hi" \o/`},
	{Op: OpSet, Key: "unicode ключ", Value: "значение ✓ 🙂"},
	{Op: OpSet, Key: "empty value", Value: ""},
	{Op: OpSet, Key: "ttl", Value: "x", ExpiresAt: 1735787045123456789},
	{Op: OpDelete, Key: "a"},
	{Op: OpSet, Key: "", Value: "empty key"},
}

func TestLogRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	l := openTestLog(t, path)
	for _, r := range tricky {
		if err := l.Append(r); err != nil {
			t.Fatalf("Append(%+v) error = %v", r, err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	got := replayAll(t, openTestLog(t, path))
	if !slices.Equal(got, tricky) {
		t.Errorf("Replay after reopen returned\n  %+v\nwant\n  %+v", got, tricky)
	}
}

func TestLogFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	l := openTestLog(t, path)
	for _, r := range []Record{
		{Op: OpSet, Key: "k", Value: "v"},
		{Op: OpSet, Key: "t", Value: "a\nb", ExpiresAt: 42},
		{Op: OpDelete, Key: "k"},
	} {
		if err := l.Append(r); err != nil {
			t.Fatalf("Append error = %v", err)
		}
	}
	want := `{"op":"set","key":"k","value":"v"}` + "\n" +
		`{"op":"set","key":"t","value":"a\nb","exp":42}` + "\n" +
		`{"op":"del","key":"k"}` + "\n"
	if got := readFile(t, path); got != want {
		t.Errorf("log file contents =\n%s\nwant (one JSON object per line, see Record's struct tags)\n%s", got, want)
	}
}

func TestOpenLogCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.log")
	l := openTestLog(t, path)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("OpenLog did not create %s: %v", path, err)
	}
	if got := replayAll(t, l); len(got) != 0 {
		t.Errorf("Replay of empty log returned %d records, want 0", len(got))
	}
}

func TestOpenLogBadDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist", "kv.log")
	l, err := OpenLog(path, false)
	if err == nil {
		l.Close()
		t.Fatalf("OpenLog(%q) error = nil, want an error for a missing directory", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("OpenLog error = %v, want it to wrap os.ErrNotExist (use %%w)", err)
	}
}

func TestReplayTornTail(t *testing.T) {
	good := `{"op":"set","key":"a","value":"1"}` + "\n" +
		`{"op":"set","key":"b","value":"2"}` + "\n"
	tests := []struct {
		name string
		torn string
	}{
		{"half a record", `{"op":"set","ke`},
		{"complete JSON but no newline", `{"op":"set","key":"c","value":"3"}`},
		{"single byte", `{`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "kv.log")
			writeFile(t, path, good+tt.torn)

			l := openTestLog(t, path)
			got := replayAll(t, l)
			if len(got) != 2 {
				t.Fatalf("Replay returned %d records, want the 2 complete ones: %+v", len(got), got)
			}
			if after := readFile(t, path); after != good {
				t.Fatalf("after Replay the file is\n%q\nwant the torn tail truncated:\n%q", after, good)
			}

			// New appends must land after the good prefix, not after garbage.
			if err := l.Append(Record{Op: OpSet, Key: "d", Value: "4"}); err != nil {
				t.Fatal(err)
			}
			l.Close()
			got = replayAll(t, openTestLog(t, path))
			keys := make([]string, len(got))
			for i, r := range got {
				keys[i] = r.Key
			}
			if want := []string{"a", "b", "d"}; !slices.Equal(keys, want) {
				t.Errorf("after torn-tail repair and one Append, replayed keys = %q, want %q", keys, want)
			}
		})
	}
}

func TestReplayCorrupt(t *testing.T) {
	ok := `{"op":"set","key":"a","value":"1"}` + "\n"
	tests := []struct {
		name     string
		content  string
		wantLine string
	}{
		{"garbage in the middle", ok + "not json\n" + ok, "line 2"},
		{"complete bad last line", ok + ok + "{\"op\":\n", "line 3"},
		{"unknown op", `{"op":"incr","key":"a"}` + "\n", "line 1"},
		{"missing op", `{"key":"a","value":"1"}` + "\n", "line 1"},
		{"blank line", ok + "\n" + ok, "line 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "kv.log")
			writeFile(t, path, tt.content)
			l := openTestLog(t, path)
			err := l.Replay(func(Record) error { return nil })
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("Replay() error = %v, want an error wrapping ErrCorrupt", err)
			}
			if !strings.Contains(err.Error(), tt.wantLine) {
				t.Errorf("Replay() error = %q, want it to mention %q", err, tt.wantLine)
			}
			if after := readFile(t, path); after != tt.content {
				t.Errorf("Replay modified a corrupt log; corruption must be reported, not repaired")
			}
		})
	}
}

func TestReplayCallbackError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	l := openTestLog(t, path)
	for _, k := range []string{"a", "b", "c"} {
		if err := l.Append(Record{Op: OpSet, Key: k}); err != nil {
			t.Fatal(err)
		}
	}
	errStop := errors.New("stop here")
	calls := 0
	err := l.Replay(func(r Record) error {
		calls++
		if r.Key == "b" {
			return errStop
		}
		return nil
	})
	if !errors.Is(err, errStop) {
		t.Errorf("Replay() error = %v, want it to wrap the callback's error", err)
	}
	if calls != 2 {
		t.Errorf("callback called %d times, want 2 (Replay must stop at the first error)", calls)
	}
}

func TestLogCompact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kv.log")
	l := openTestLog(t, path)
	for range 50 {
		if err := l.Append(Record{Op: OpSet, Key: "k", Value: "old"}); err != nil {
			t.Fatal(err)
		}
	}

	snapshot := []Record{{Op: OpSet, Key: "k", Value: "new"}, {Op: OpSet, Key: "z", Value: "1", ExpiresAt: 7}}
	if err := l.Compact(snapshot); err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	if err := l.Append(Record{Op: OpDelete, Key: "z"}); err != nil {
		t.Fatalf("Append after Compact error = %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	want := append(slices.Clone(snapshot), Record{Op: OpDelete, Key: "z"})
	if got := replayAll(t, openTestLog(t, path)); !slices.Equal(got, want) {
		t.Errorf("after Compact + Append, Replay = %+v, want %+v (did Append write to the old file?)", got, want)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "kv.log" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %q after Compact, want only [\"kv.log\"] (temp file left behind?)", names)
	}
}

func TestLogCompactEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	l := openTestLog(t, path)
	if err := l.Append(Record{Op: OpSet, Key: "k"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Compact(nil); err != nil {
		t.Fatalf("Compact(nil) error = %v", err)
	}
	if got := readFile(t, path); got != "" {
		t.Errorf("file after Compact(nil) = %q, want empty", got)
	}
}

func TestLogClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	l := openTestLog(t, path)
	if err := l.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("second Close() error = %v, want nil (Close should be idempotent)", err)
	}
	if err := l.Append(Record{Op: OpSet, Key: "k"}); !errors.Is(err, ErrClosed) {
		t.Errorf("Append after Close error = %v, want ErrClosed", err)
	}
	if err := l.Compact(nil); !errors.Is(err, ErrClosed) {
		t.Errorf("Compact after Close error = %v, want ErrClosed", err)
	}
	if err := l.Replay(func(Record) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Errorf("Replay after Close error = %v, want ErrClosed", err)
	}
}

func TestLogSyncWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	l, err := OpenLog(path, true)
	if err != nil {
		t.Fatalf("OpenLog(sync=true) error = %v", err)
	}
	defer l.Close()
	if err := l.Append(Record{Op: OpSet, Key: "k", Value: "v"}); err != nil {
		t.Fatalf("Append with sync error = %v", err)
	}
	if got := readFile(t, path); got != `{"op":"set","key":"k","value":"v"}`+"\n" {
		t.Errorf("file = %q", got)
	}
}
