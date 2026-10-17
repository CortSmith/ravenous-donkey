package store

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

// fakeClock is a manually advanced clock, safe for concurrent use.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// openDB opens a DB at path with clk and registers Close.
func openDB(t *testing.T, path string, clk *fakeClock) *DB {
	t.Helper()
	db, err := Open(path, Options{Now: clk.Now})
	if err != nil {
		t.Fatalf("Open(%q) error = %v", path, err)
	}
	if db == nil {
		t.Fatalf("Open(%q) returned nil *DB and nil error", path)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// reopen closes db and opens a new DB on the same path.
func reopen(t *testing.T, db *DB, path string, clk *fakeClock) *DB {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return openDB(t, path, clk)
}

func mustSet(t *testing.T, db *DB, key, value string, ttl time.Duration) {
	t.Helper()
	if err := db.Set(key, value, ttl); err != nil {
		t.Fatalf("Set(%q, %q, %v) error = %v", key, value, ttl, err)
	}
}

// contents returns every live key/value pair in db.
func contents(t *testing.T, db *DB) map[string]string {
	t.Helper()
	m := make(map[string]string)
	for _, k := range db.Keys() {
		v, err := db.Get(k)
		if err != nil {
			t.Fatalf("Keys() listed %q but Get(%q) error = %v", k, k, err)
		}
		m[k] = v
	}
	return m
}

func wantContents(t *testing.T, db *DB, want map[string]string) {
	t.Helper()
	if got := contents(t, db); !maps.Equal(got, want) {
		t.Errorf("DB contents = %v, want %v", got, want)
	}
}

// lineCount returns the number of lines in the file at path.
func lineCount(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "\n")
}

func TestDBPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	clk := &fakeClock{now: t0}
	db := openDB(t, path, clk)

	mustSet(t, db, "a", "1", 0)
	mustSet(t, db, "b", "2", 0)
	mustSet(t, db, "c", "3", 0)
	mustSet(t, db, "c", "33", 0)
	mustSet(t, db, "multi", "line\nvalue", 0)
	if ok, err := db.Delete("b"); err != nil || !ok {
		t.Fatalf("Delete(\"b\") = (%v, %v), want (true, nil)", ok, err)
	}
	want := map[string]string{"a": "1", "c": "33", "multi": "line\nvalue"}
	wantContents(t, db, want)

	db = reopen(t, db, path, clk)
	wantContents(t, db, want)
}

func TestDBDeleteMissingWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	clk := &fakeClock{now: t0}
	db := openDB(t, path, clk)
	mustSet(t, db, "a", "1", 0)
	before := lineCount(t, path)

	ok, err := db.Delete("nope")
	if err != nil || ok {
		t.Errorf("Delete(\"nope\") = (%v, %v), want (false, nil)", ok, err)
	}
	if after := lineCount(t, path); after != before {
		t.Errorf("deleting a missing key grew the log from %d to %d lines; it should write nothing", before, after)
	}
}

func TestDBTTLSurvivesRestart(t *testing.T) {
	tests := []struct {
		name    string
		advance time.Duration // time that passes while the process is "down"
		live    bool
	}{
		{"restart before expiry", 5 * time.Second, true},
		{"restart just before expiry", 10*time.Second - time.Nanosecond, true},
		{"restart at expiry", 10 * time.Second, false},
		{"restart long after expiry", time.Hour, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "kv.log")
			clk := &fakeClock{now: t0}
			db := openDB(t, path, clk)
			mustSet(t, db, "session", "abc", 10*time.Second)
			mustSet(t, db, "forever", "x", 0)

			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			clk.Advance(tt.advance)
			db = openDB(t, path, clk)

			_, err := db.Get("session")
			if tt.live && err != nil {
				t.Errorf("after restart at +%v, Get(\"session\") error = %v, want value (expiry must be stored as an absolute time)", tt.advance, err)
			}
			if !tt.live && !errors.Is(err, ErrNotFound) {
				t.Errorf("after restart at +%v, Get(\"session\") error = %v, want ErrNotFound", tt.advance, err)
			}
			if v, err := db.Get("forever"); err != nil || v != "x" {
				t.Errorf("Get(\"forever\") = (%q, %v), want (\"x\", nil)", v, err)
			}
		})
	}
}

func TestDBExpiredRecordHidesOlderValue(t *testing.T) {
	// set k=v1 (no TTL), then set k=v2 with a TTL that has passed by the time
	// we restart. Replay must leave k absent, not resurrect v1.
	path := filepath.Join(t.TempDir(), "kv.log")
	clk := &fakeClock{now: t0}
	db := openDB(t, path, clk)
	mustSet(t, db, "k", "v1", 0)
	mustSet(t, db, "k", "v2", time.Second)
	clk.Advance(2 * time.Second)

	db = reopen(t, db, path, clk)
	if v, err := db.Get("k"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(\"k\") = (%q, %v), want ErrNotFound: the expired v2 overwrote v1", v, err)
	}
}

func TestDBCompact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	clk := &fakeClock{now: t0}
	db := openDB(t, path, clk)

	for i := range 100 {
		mustSet(t, db, "counter", fmt.Sprint(i), 0)
	}
	for i := range 10 {
		mustSet(t, db, fmt.Sprintf("tmp%d", i), "x", 0)
		if _, err := db.Delete(fmt.Sprintf("tmp%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	mustSet(t, db, "expires-soon", "x", time.Second)
	mustSet(t, db, "expires-later", "y", time.Hour)
	clk.Advance(time.Minute)

	before := lineCount(t, path)
	if err := db.Compact(); err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	after := lineCount(t, path)
	if after != 2 {
		t.Errorf("log has %d lines after Compact (was %d), want 2: one per live key", after, before)
	}

	want := map[string]string{"counter": "99", "expires-later": "y"}
	wantContents(t, db, want)

	// Writes after compaction must be durable too.
	mustSet(t, db, "post", "compact", 0)
	want["post"] = "compact"

	db = reopen(t, db, path, clk)
	wantContents(t, db, want)

	// The TTL of a compacted key must survive compaction.
	clk.Advance(time.Hour)
	db = reopen(t, db, path, clk)
	delete(want, "expires-later")
	wantContents(t, db, want)
}

func TestDBCompactOutputIsSorted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	clk := &fakeClock{now: t0}
	db := openDB(t, path, clk)
	for _, k := range []string{"m", "c", "x", "a"} {
		mustSet(t, db, k, k, 0)
	}
	if err := db.Compact(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"op":"set","key":"a","value":"a"}` + "\n" +
		`{"op":"set","key":"c","value":"c"}` + "\n" +
		`{"op":"set","key":"m","value":"m"}` + "\n" +
		`{"op":"set","key":"x","value":"x"}` + "\n"
	if string(b) != want {
		t.Errorf("compacted log =\n%s\nwant (sorted by key, deterministic)\n%s", b, want)
	}
}

func TestDBOpenCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	content := `{"op":"set","key":"a","value":"1"}` + "\n" + "garbage\n" + `{"op":"set","key":"b","value":"2"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := Open(path, Options{})
	if err == nil {
		db.Close()
		t.Fatal("Open on a corrupt log error = nil, want an error wrapping ErrCorrupt")
	}
	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("Open error = %v, want it to wrap ErrCorrupt", err)
	}
}

func TestDBOpenTornTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	content := `{"op":"set","key":"a","value":"1"}` + "\n" + `{"op":"set","key":"b","val`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	db := openDB(t, path, &fakeClock{now: t0})
	wantContents(t, db, map[string]string{"a": "1"})
}

func TestDBClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	db := openDB(t, path, &fakeClock{now: t0})
	mustSet(t, db, "a", "1", 0)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Errorf("second Close() error = %v, want nil", err)
	}
	if err := db.Set("b", "2", 0); !errors.Is(err, ErrClosed) {
		t.Errorf("Set after Close error = %v, want ErrClosed", err)
	}
	if _, err := db.Delete("a"); !errors.Is(err, ErrClosed) {
		t.Errorf("Delete after Close error = %v, want ErrClosed", err)
	}
	if err := db.Compact(); !errors.Is(err, ErrClosed) {
		t.Errorf("Compact after Close error = %v, want ErrClosed", err)
	}
}

func TestDBSyncOption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	db, err := Open(path, Options{Sync: true})
	if err != nil {
		t.Fatalf("Open(Sync: true) error = %v", err)
	}
	if err := db.Set("k", "v", time.Hour); err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path, Options{}) // real clock: the key has an hour left
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if v, err := db.Get("k"); err != nil || v != "v" {
		t.Errorf("Get(\"k\") = (%q, %v), want (\"v\", nil)", v, err)
	}
}

// TestDBConcurrent mixes writers, readers and compactions. Run with -race.
// Afterwards, the state recovered from disk must equal the in-memory state.
func TestDBConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.log")
	clk := &fakeClock{now: t0}
	db := openDB(t, path, clk)

	const workers, ops = 8, 200
	var wg sync.WaitGroup
	errs := make(chan error, workers+1)
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ops {
				key := fmt.Sprintf("k%d", (w+i)%20)
				var err error
				switch i % 4 {
				case 0, 1:
					err = db.Set(key, fmt.Sprintf("w%d-i%d", w, i), 0)
				case 2:
					_, err = db.Delete(key)
				case 3:
					_, _ = db.Get(key)
					_ = db.Keys()
				}
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 20 {
			if err := db.Compact(); err != nil {
				errs <- fmt.Errorf("Compact: %w", err)
				return
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	want := contents(t, db)
	db = reopen(t, db, path, clk)
	if got := contents(t, db); !maps.Equal(got, want) {
		t.Errorf("state after reopen differs from state before close\n got: %v\nwant: %v\nkeys only in one: %q",
			got, want, symmetricDiff(got, want))
	}
}

func symmetricDiff(a, b map[string]string) []string {
	var d []string
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			d = append(d, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			d = append(d, k)
		}
	}
	slices.Sort(d)
	return d
}
