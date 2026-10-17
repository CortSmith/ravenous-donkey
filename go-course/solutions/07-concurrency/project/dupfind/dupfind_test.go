package dupfind

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Run with: go test -race ./modules/07-concurrency/project/...

const timeout = 10 * time.Second

// writeFiles creates files under a fresh temp dir. Keys are slash-separated
// relative paths; values are contents. It returns the temp dir.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// paths joins each slash-separated name onto dir.
func paths(dir string, names ...string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = filepath.Join(dir, filepath.FromSlash(n))
	}
	slices.Sort(out)
	return out
}

// find calls Find with a timeout so a deadlocked worker pool fails the test
// instead of hanging the test binary.
func find(t *testing.T, ctx context.Context, root string, opts Options) ([]Group, error) {
	t.Helper()
	type result struct {
		g   []Group
		err error
	}
	ch := make(chan result, 1)
	go func() {
		g, err := Find(ctx, root, opts)
		ch <- result{g, err}
	}()
	select {
	case r := <-ch:
		return r.g, r.err
	case <-time.After(timeout):
		t.Fatalf("Find did not return within %v (deadlock in the worker pool?)", timeout)
		return nil, nil
	}
}

func checkNoLeak(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			n := runtime.Stack(buf, true)
			t.Errorf("goroutine leak: %d goroutines, want <= %d\n%s", runtime.NumGoroutine(), base, buf[:n])
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHashFile(t *testing.T) {
	dir := writeFiles(t, map[string]string{"hello.txt": "hello\n", "empty": ""})
	tests := []struct {
		name string
		want string
	}{
		{"hello.txt", "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"},
		{"empty", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	}
	for _, tt := range tests {
		got, err := HashFile(t.Context(), filepath.Join(dir, tt.name))
		if err != nil || got != tt.want {
			t.Errorf("HashFile(%q) = %q, %v; want %q, nil", tt.name, got, err, tt.want)
		}
	}
	if _, err := HashFile(t.Context(), filepath.Join(dir, "missing")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("HashFile(missing) error = %v, want fs.ErrNotExist", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := HashFile(ctx, filepath.Join(dir, "hello.txt")); !errors.Is(err, context.Canceled) {
		t.Errorf("HashFile(canceled ctx) error = %v, want context.Canceled", err)
	}
}

func TestFind(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		opts  Options
		want  func(dir string) []Group
	}{
		{
			name:  "empty directory",
			files: map[string]string{},
			want:  func(string) []Group { return nil },
		},
		{
			name:  "no duplicates",
			files: map[string]string{"a": "one", "b": "two", "c": "three"},
			want:  func(string) []Group { return nil },
		},
		{
			name: "one pair across directories",
			files: map[string]string{
				"a.txt":         "hello",
				"sub/deep/b.md": "hello",
				"c.txt":         "world", // same size, different content
				"d.txt":         "unique size",
			},
			want: func(dir string) []Group {
				return []Group{{Size: 5, Hash: sha("hello"), Paths: paths(dir, "a.txt", "sub/deep/b.md")}}
			},
		},
		{
			name: "three-way group and a pair, sorted by size descending",
			files: map[string]string{
				"x1": "abc", "x2": "abc", "x3": "abc",
				"big1": "0123456789", "big2": "0123456789",
				"other": "xyz",
			},
			want: func(dir string) []Group {
				return []Group{
					{Size: 10, Hash: sha("0123456789"), Paths: paths(dir, "big1", "big2")},
					{Size: 3, Hash: sha("abc"), Paths: paths(dir, "x1", "x2", "x3")},
				}
			},
		},
		{
			name: "two groups of the same size sorted by first path",
			files: map[string]string{
				"m/1": "aaaa", "z/1": "aaaa",
				"b/1": "bbbb", "b/2": "bbbb",
			},
			want: func(dir string) []Group {
				return []Group{
					{Size: 4, Hash: sha("bbbb"), Paths: paths(dir, "b/1", "b/2")},
					{Size: 4, Hash: sha("aaaa"), Paths: paths(dir, "m/1", "z/1")},
				}
			},
		},
		{
			name:  "empty files are ignored",
			files: map[string]string{"e1": "", "e2": "", "e3": ""},
			want:  func(string) []Group { return nil },
		},
		{
			name:  "MinSize filters small files",
			files: map[string]string{"s1": "hi", "s2": "hi", "l1": "hello world", "l2": "hello world"},
			opts:  Options{MinSize: 3},
			want: func(dir string) []Group {
				return []Group{{Size: 11, Hash: sha("hello world"), Paths: paths(dir, "l1", "l2")}}
			},
		},
		{
			name:  "single worker",
			files: map[string]string{"a": "same", "b": "same", "c": "same"},
			opts:  Options{Workers: 1},
			want: func(dir string) []Group {
				return []Group{{Size: 4, Hash: sha("same"), Paths: paths(dir, "a", "b", "c")}}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := runtime.NumGoroutine()
			dir := writeFiles(t, tt.files)
			got, err := find(t, t.Context(), dir, tt.opts)
			if err != nil {
				t.Fatalf("Find error = %v, want nil", err)
			}
			if want := tt.want(dir); !reflect.DeepEqual(got, want) {
				t.Errorf("Find() =\n  %+v\nwant\n  %+v", got, want)
			}
			checkNoLeak(t, base)
		})
	}
}

func TestFindIgnoresSymlinks(t *testing.T) {
	dir := writeFiles(t, map[string]string{"real": "content", "other": "content"})
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	got, err := find(t, t.Context(), dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Group{{Size: 7, Hash: sha("content"), Paths: paths(dir, "other", "real")}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Find() = %+v, want %+v (symlinks must be skipped)", got, want)
	}
}

func TestFindOnlyHashesSizeCollisions(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"a": "1", "b": "22", "c": "333", // unique sizes: never hashed
		"d": "4444", "e": "5555", // same size, different content: hashed
		"f": "666666", "g": "666666", // duplicates: hashed
	})
	var mu sync.Mutex
	var hashed []string
	opts := Options{Hash: func(ctx context.Context, path string) (string, error) {
		mu.Lock()
		hashed = append(hashed, filepath.Base(path))
		mu.Unlock()
		return HashFile(ctx, path)
	}}
	if _, err := find(t, t.Context(), dir, opts); err != nil {
		t.Fatal(err)
	}
	slices.Sort(hashed)
	if want := []string{"d", "e", "f", "g"}; !slices.Equal(hashed, want) {
		t.Errorf("hashed files %v, want exactly %v (only files sharing a size need hashing)", hashed, want)
	}
}

func TestFindUsesCustomHash(t *testing.T) {
	dir := writeFiles(t, map[string]string{"a": "xx", "b": "yy"})
	// A deliberately terrible hash: everything collides. Find must trust it.
	opts := Options{Hash: func(context.Context, string) (string, error) { return "same", nil }}
	got, err := find(t, t.Context(), dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := []Group{{Size: 2, Hash: "same", Paths: paths(dir, "a", "b")}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Find() = %+v, want %+v (Options.Hash must be used when set)", got, want)
	}
}

func TestFindBoundsWorkers(t *testing.T) {
	files := map[string]string{}
	for i := range 24 {
		files[string(rune('a'+i))] = "same size " + string(rune('a'+i))
	}
	dir := writeFiles(t, files)
	for _, workers := range []int{1, 3, 6} {
		t.Run("workers="+strconv.Itoa(workers), func(t *testing.T) {
			var active, peak atomic.Int32
			full := make(chan struct{})
			var once sync.Once
			opts := Options{Workers: workers, Hash: func(ctx context.Context, path string) (string, error) {
				n := active.Add(1)
				defer active.Add(-1)
				for {
					p := peak.Load()
					if n <= p || peak.CompareAndSwap(p, n) {
						break
					}
				}
				if int(n) == workers {
					once.Do(func() { close(full) })
				}
				select {
				case <-full:
				case <-time.After(300 * time.Millisecond):
				}
				return HashFile(ctx, path)
			}}
			if _, err := find(t, t.Context(), dir, opts); err != nil {
				t.Fatal(err)
			}
			if got := int(peak.Load()); got > workers {
				t.Errorf("peak concurrent hashes = %d, want <= %d (Workers must bound concurrency)", got, workers)
			} else if got < workers {
				t.Errorf("peak concurrent hashes = %d, want %d (hashing is not concurrent)", got, workers)
			}
		})
	}
}

var errDisk = errors.New("disk on fire")

func TestFindHashError(t *testing.T) {
	base := runtime.NumGoroutine()
	files := map[string]string{}
	for i := range 40 {
		files[string(rune('A'+i))] = "0123456789"
	}
	dir := writeFiles(t, files)
	var calls atomic.Int32
	opts := Options{Workers: 4, Hash: func(ctx context.Context, path string) (string, error) {
		if calls.Add(1) == 3 {
			return "", errDisk
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Millisecond):
			return "h", nil
		}
	}}
	got, err := find(t, t.Context(), dir, opts)
	if !errors.Is(err, errDisk) {
		t.Fatalf("Find error = %v, want it to wrap errDisk (the first error, not context.Canceled)", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("Find error = %q, want it to name the failing file's path", err)
	}
	if got != nil {
		t.Errorf("Find groups = %+v, want nil on error", got)
	}
	if c := calls.Load(); c > 20 {
		t.Errorf("Hash called %d times for 40 files after failing on call 3, want far fewer (stop hashing after an error)", c)
	}
	checkNoLeak(t, base)
}

func TestFindCanceled(t *testing.T) {
	base := runtime.NumGoroutine()
	dir := writeFiles(t, map[string]string{"a": "dup", "b": "dup"})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := find(t, ctx, dir, Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("Find(canceled ctx) error = %v, want context.Canceled", err)
	}
	checkNoLeak(t, base)
}

func TestFindMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := find(t, t.Context(), root, Options{}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Find(missing root) error = %v, want fs.ErrNotExist", err)
	}
}

func TestWasted(t *testing.T) {
	tests := []struct {
		groups []Group
		want   int64
	}{
		{nil, 0},
		{[]Group{{Size: 10, Paths: []string{"a", "b"}}}, 10},
		{[]Group{{Size: 10, Paths: []string{"a", "b", "c"}}, {Size: 3, Paths: []string{"d", "e"}}}, 23},
	}
	for _, tt := range tests {
		if got := Wasted(tt.groups); got != tt.want {
			t.Errorf("Wasted(%+v) = %d, want %d", tt.groups, got, tt.want)
		}
	}
}

func TestWriteReport(t *testing.T) {
	groups := []Group{
		{Size: 10, Hash: sha("0123456789"), Paths: []string{"/d/big1", "/d/big2"}},
		{Size: 3, Hash: "abc", Paths: []string{"/d/x1", "/d/x2", "/d/x3"}},
	}
	want := `group 1: 2 files, 10 bytes each, hash 84d89877f0d4
  /d/big1
  /d/big2
group 2: 3 files, 3 bytes each, hash abc
  /d/x1
  /d/x2
  /d/x3
2 groups, 16 bytes reclaimable
`
	var buf bytes.Buffer
	if err := WriteReport(&buf, groups); err != nil {
		t.Fatalf("WriteReport error = %v", err)
	}
	if got := buf.String(); got != want {
		t.Errorf("WriteReport output:\n%s\nwant:\n%s", got, want)
	}

	buf.Reset()
	if err := WriteReport(&buf, nil); err != nil {
		t.Fatalf("WriteReport(nil) error = %v", err)
	}
	if got, want := buf.String(), "0 groups, 0 bytes reclaimable\n"; got != want {
		t.Errorf("WriteReport(nil) = %q, want %q", got, want)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errDisk }

func TestWriteReportPropagatesWriteErrors(t *testing.T) {
	groups := []Group{{Size: 1, Hash: "h", Paths: []string{"a", "b"}}}
	if err := WriteReport(failWriter{}, groups); !errors.Is(err, errDisk) {
		t.Errorf("WriteReport(failing writer) error = %v, want errDisk", err)
	}
}
