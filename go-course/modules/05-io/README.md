# Module 05: Readers, Writers, Files & Encoding

## Goals

By the end of this module, you'll be able to:

- Write functions that operate on abstract streams using `io.Reader` and `io.Writer`
- Compose readers and writers to build complex pipelines (e.g., with `io.TeeReader`, `io.MultiWriter`, `io.Copy`)
- Handle unbounded input using `bufio.Scanner` while respecting its 64 KiB limit and recovering from oversized lines
- Read files safely, choosing between whole-file loading and streaming approaches
- Encode and decode CSV and JSON data with proper error handling
- Work with custom readers and writers to implement domain-specific transformations
- Design APIs that respect the `io.Reader`/`io.Writer` abstraction and compose with the standard library

## Concepts

### The io.Reader and io.Writer Abstractions

At the heart of Go's I/O design are two simple interfaces:

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

// Anything with the method is one; nothing to declare:
var r Reader = strings.NewReader("hello\n")
var w Writer = os.Stdout
p := make([]byte, 16)
n, _ := r.Read(p)
w.Write(p[:n]) // hello
```

These abstractions let you write code that works with any source or sink: files, network connections, buffers, pipes, or custom implementations. The key insight is to write functions *against the interface*, not against a concrete type like `*os.File`.

**Why this matters:** A function that accepts `io.Reader` can be tested with a `strings.Reader`, can be used with a file, and can be composed with other readers. A function that accepts `*os.File` can only be used with files, and is much harder to test.

```go
import "crypto/rand"

// Good: works with any reader
func Count(r io.Reader) (int, error) {
	b, err := io.ReadAll(r)
	return len(b), err
}

// Bad: only works with files, hard to test
func CountFile(f *os.File) (int, error) { return Count(f) }

func main() {
	f, _ := os.CreateTemp("", "count-*.txt")
	defer os.Remove(f.Name())
	f.WriteString("file contents")
	f.Seek(0, io.SeekStart)

	fmt.Println(Count(f))                                // works with a file: 13 <nil>
	fmt.Println(Count(strings.NewReader("test")))        // also works: 4 <nil>
	fmt.Println(Count(io.LimitReader(rand.Reader, 10))) // and any other reader: 10 <nil>
}
```

### Read Semantics: `n` and `err` Together

One of the trickiest parts of `io.Reader` is that `Read` may return both `n > 0` **and** `err != nil` in the same call. This happens at EOF: the last chunk may be returned together with `io.EOF`. Always process the n bytes first, then look at the error.

```go
// A reader that returns data AND io.EOF in the same call, as Read may.
type lastChunk struct{ done bool }

func (l *lastChunk) Read(p []byte) (int, error) {
	if l.done {
		return 0, io.EOF
	}
	l.done = true
	return copy(p, "last bytes"), io.EOF
}

// Common mistake: ignoring n when err != nil
func readWrong(r io.Reader) (got []byte, err error) {
	p := make([]byte, 64)
	for {
		n, err := r.Read(p)
		if err != nil {
			return got, err // WRONG: you lose the n bytes that were read
		}
		got = append(got, p[:n]...)
	}
}

// Correct
func readRight(r io.Reader) (got []byte, err error) {
	p := make([]byte, 64)
	for {
		n, err := r.Read(p)
		if n > 0 {
			got = append(got, p[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				return got, nil
			}
			return got, err
		}
	}
}

wrong, _ := readWrong(&lastChunk{})
right, _ := readRight(&lastChunk{})
fmt.Printf("wrong: %q, right: %q\n", wrong, right) // wrong: "", right: "last bytes"

// Or use helpers
all, err := io.ReadAll(&lastChunk{}) // handles n/err correctly
fmt.Printf("%q %v\n", all, err)      // "last bytes" <nil>
p := make([]byte, 4)
n, err := io.ReadFull(strings.NewReader("abcdef"), p) // read exactly len(p) bytes or error
fmt.Println(n, err, string(p))                         // 4 <nil> abcd
m, err := io.CopyN(os.Stdout, strings.NewReader("0123456789"), 3) // read/write exactly 3 bytes
fmt.Println("", m, err)                                           // 012 3 <nil>
```

### Composing Readers and Writers

The standard library provides composable building blocks:

```go
// io.Copy copies from src to dst until src returns io.EOF:
//
//	func Copy(dst Writer, src Reader) (written int64, err error)
n, err := io.Copy(os.Stdout, strings.NewReader("copied\n"))
fmt.Println(n, err) // 7 <nil>

// io.MultiReader concatenates several readers
multiR := io.MultiReader(strings.NewReader("one "), strings.NewReader("two "), strings.NewReader("three\n"))
io.Copy(os.Stdout, multiR) // one two three

// io.MultiWriter broadcasts writes to multiple writers
var buf1, buf2 bytes.Buffer
multiW := io.MultiWriter(&buf1, &buf2, os.Stdout)
fmt.Fprintf(multiW, "goes to all three\n")
fmt.Println(buf1.Len(), buf2.Len()) // 18 18

// io.TeeReader reads from src, writes to dst, and returns a reader
// that gives you the same bytes that were written to dst
var dst bytes.Buffer
tee := io.TeeReader(strings.NewReader("teed"), &dst)
data, _ := io.ReadAll(tee) // data == what was written to dst
fmt.Println(string(data), dst.String()) // teed teed

// io.LimitReader returns a reader that stops after n bytes
limited := io.LimitReader(strings.NewReader("0123456789"), 4) // read at most 4 bytes
data, _ = io.ReadAll(limited)
fmt.Println(string(data)) // 0123

// io.NewSectionReader reads a window of an io.ReaderAt (a file, a
// strings.Reader): here 5 bytes starting at offset 6
section := io.NewSectionReader(strings.NewReader("hello, world"), 6, 5)
data, _ = io.ReadAll(section)
fmt.Printf("%q\n", data) // " worl"
```

These compose to build powerful pipelines with minimal copying:

```go
dir, _ := os.MkdirTemp("", "tee")
defer os.RemoveAll(dir)
name := filepath.Join(dir, "data.txt")
os.WriteFile(name, []byte(strings.Repeat("compress me ", 1000)), 0o644)

// Compress a file, write both the original and compressed to separate files
src, err := os.Open(name)
if err != nil {
	log.Fatal(err)
}
defer src.Close()

orig, _ := os.Create(name + ".orig")
defer orig.Close()

gzipped, _ := os.Create(name + ".gz")
defer gzipped.Close()

// Use TeeReader to observe the data flow, then compress
tee := io.TeeReader(src, orig)
gz := gzip.NewWriter(gzipped)

n, err := io.Copy(gz, tee) // reads from src, writes to both orig and gzipped
if err := gz.Close(); err != nil { // Close flushes the last compressed block
	log.Fatal(err)
}
o, _ := orig.Stat()
g, _ := gzipped.Stat()
fmt.Println(n, err, o.Size(), g.Size()) // 12000 <nil> 12000, and far fewer bytes compressed
```

### Buffered I/O with bufio

For character-at-a-time or line-at-a-time reading, the `bufio` package provides buffering:

#### bufio.Scanner

`Scanner` reads lines or other tokens without holding the entire input in memory. It handles line ending conversion and raises an error if a line exceeds `bufio.MaxScanTokenSize` (64 KiB).

```go
r := strings.NewReader("first line\nsecond line\nno newline at the end")
sc := bufio.NewScanner(r)
for sc.Scan() {
	line := sc.Text() // a string, without the newline
	fmt.Printf("%q\n", line)
}
if err := sc.Err(); err != nil {
	log.Fatal(err) // NEVER skip this check
}
```

The 64 KiB limit is intentional: it prevents memory exhaustion on malformed input. If you need longer lines, either:

1. Raise the limit with `sc.Buffer`:
   ```go
   long := strings.Repeat("x", 100_000) // one 100 KB line: over the default 64 KiB limit
   sc := bufio.NewScanner(strings.NewReader(long))
   for sc.Scan() {
   }
   fmt.Println(sc.Err()) // bufio.Scanner: token too long

   sc = bufio.NewScanner(strings.NewReader(long))
   sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // max 10 MiB per line
   for sc.Scan() {
   	fmt.Println(len(sc.Text())) // 100000
   }
   fmt.Println(sc.Err()) // <nil>
   ```

2. Use `bufio.Reader.ReadLine` instead, which is more flexible:
   ```go
   long := strings.Repeat("x", 40) + "\nshort\n"
   br := bufio.NewReaderSize(strings.NewReader(long), 16) // tiny buffer to force splits
   for {
   	chunk, isPrefix, err := br.ReadLine() // isPrefix true means the line continues
   	if err != nil {
   		break // io.EOF
   	}
   	fmt.Printf("%q isPrefix=%v\n", chunk, isPrefix)
   }
   ```

3. Read in fixed-size chunks yourself and search with `bytes.IndexByte`.

#### bufio.Writer

A `Writer` buffers writes for efficiency. You must call `Flush` before closing to ensure all data is written.

```go
f, err := os.CreateTemp("", "buffered-*.txt")
if err != nil {
	log.Fatal(err)
}
defer os.Remove(f.Name())
defer f.Close()

w := bufio.NewWriter(f)
fmt.Fprintf(w, "buffered output\n")
if err := w.Flush(); err != nil {
	log.Fatal(err)
}
data, _ := os.ReadFile(f.Name())
fmt.Printf("%q\n", data) // "buffered output\n"
```

In a CLI, remember that `os.Exit` does not run defers, so buffered output is lost if you exit without flushing.

### File Operations and Error Handling

#### Whole-file I/O

For small or bounded files, `os.ReadFile` and `os.WriteFile` are simple and safe:

```go
dir, _ := os.MkdirTemp("", "readfile")
defer os.RemoveAll(dir)
os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"debug": true}`), 0o644)

data, err := os.ReadFile(filepath.Join(dir, "config.json"))
if err != nil {
	log.Fatal(err)
}

err = os.WriteFile(filepath.Join(dir, "output.txt"), data, 0o644)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("copied %d bytes: %s\n", len(data), data)
```

#### Streaming and Cleanup

For large or untrusted input, stream with `io.Reader`/`io.Writer`. Always close files and check the error:

```go
// For reading: Close error is usually safe to ignore
func firstByte(path string) (byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	b := make([]byte, 1)
	_, err = f.Read(b)
	return b[0], err
}

// For writing: Close error can hide data loss. Check it.
func write(path string, data []byte) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	w := bufio.NewWriter(f)
	if _, err := w.Write(data); err != nil {
		return err
	}
	return w.Flush()
}

path := filepath.Join(os.TempDir(), "close-demo.txt")
defer os.Remove(path)
fmt.Println(write(path, []byte("hello"))) // <nil>
b, err := firstByte(path)
fmt.Printf("%c %v\n", b, err) // h <nil>
```

For atomic replacements (preventing partial writes on crash), write to a temp file first, then rename:

```go
dir, _ := os.MkdirTemp("", "atomic")
defer os.RemoveAll(dir)
finalPath := filepath.Join(dir, "config.json")

f, _ := os.CreateTemp(dir, "tmp-") // same directory, so Rename stays on one filesystem
f.WriteString(`{"version": 2}`)
f.Close()
os.Rename(f.Name(), finalPath) // readers see the old file or the new one, never half of it

data, _ := os.ReadFile(finalPath)
fmt.Println(string(data))
```

#### Protecting Against Resource Exhaustion

When accepting untrusted input, limit its size:

```go
r := strings.NewReader(strings.Repeat("a", 5_000_000)) // pretend: untrusted, possibly huge

// Bad: io.ReadAll on untrusted input can exhaust memory
// data, _ := io.ReadAll(r)

// Good: limit to 1 MiB
limited := io.LimitReader(r, 1024*1024)
data, _ := io.ReadAll(limited)
fmt.Println(len(data)) // 1048576

// For HTTP request bodies, http.MaxBytesReader is built-in
h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	_, err := io.ReadAll(r.Body)
	fmt.Println(err) // http: request body too large
})
h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("a", 2_000_000))))
```

### Walking Directory Trees

The `filepath` package provides `WalkDir` for traversing directories without excessive allocations:

```go
dir, _ := os.MkdirTemp("", "walk")
defer os.RemoveAll(dir)
os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755)
os.WriteFile(filepath.Join(dir, "a", "one.txt"), nil, 0o644)
os.WriteFile(filepath.Join(dir, "a", "b", "two.txt"), nil, 0o644)

err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err // permission denied, etc.
	}
	if d.IsDir() {
		return nil // keep walking; print only files
	}
	rel, _ := filepath.Rel(dir, path)
	fmt.Println(rel)
	return nil
})
fmt.Println(err) // a/b/two.txt, a/one.txt (lexical order), then <nil>
```

Use `WalkDir`, not the older `Walk`: it's more efficient because `DirEntry` caches stat info.

### encoding/csv

The `csv` package reads and writes CSV with proper quoting and escape handling:

```go
import "encoding/csv"

func readCSV(f io.Reader) error {
	r := csv.NewReader(f)
	r.FieldsPerRecord = 2 // enforce 2 fields per row, or -1 for variable
	for {
		record, err := r.Read() // []string, one per field
		if err == io.EOF {
			break
		}
		if err != nil {
			return err // *csv.ParseError has Line and Column fields
		}
		fmt.Printf("%q\n", record)
	}
	return nil
}

func main() {
	// Reading
	err := readCSV(strings.NewReader("name,age\n\"Lovelace, Ada\",36\nBob\n"))
	fmt.Println(err) // record on line 3: wrong number of fields

	// Writing
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{"name", "age"})
	w.Write([]string{"Alice, Jr.", "31"}) // quoted automatically
	w.Flush()                            // MUST flush before closing
	if err := w.Error(); err != nil {
		log.Fatal(err)
	}
}
```

Key points:
- `ReadAll` loads everything into memory; use `Read` in a loop for streaming.
- Set `LazyQuotes = true` to tolerate malformed input.
- `FieldsPerRecord = 0` to set the field count based on the first record.
- `ReuseRecord = true` reduces allocations but then you must copy any record you keep.
- Don't forget `Flush` and `w.Error()`.

### encoding/json

JSON encoding and decoding in Go:

#### Tags

Struct fields with `json` tags control marshaling and unmarshaling:

```go
type User struct {
    ID       int       `json:"id"`
    Name     string    `json:"name"`
    Email    string    `json:"email,omitempty"`        // omit if empty
    Created  time.Time `json:"created,omitzero"`       // 1.24: omit if zero value
    Internal string    `json:"-"`                      // always omit
    Balance  float64   `json:"balance,string"`         // read/write as JSON string
}

out, _ := json.Marshal(User{ID: 1, Name: "Ada", Internal: "secret", Balance: 9.5})
fmt.Println(string(out)) // {"id":1,"name":"Ada","balance":"9.5"}: no email, created or internal
```

Fields without a tag use their struct name. When decoding, keys match field names case-insensitively, so `"NAME"` fills `Name`. Unexported fields are always ignored.

#### Unmarshaling

`json.Unmarshal` is for data you already have as `[]byte`. `json.NewDecoder(r).Decode(&v)` is for streams:

```go
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Whole buffer
var u User
err := json.Unmarshal([]byte(`{"id": 1, "name": "Ada"}`), &u)
fmt.Println(u, err) // {1 Ada} <nil>

// Stream (e.g., HTTP body, file)
dec := json.NewDecoder(strings.NewReader(`{"id": 2, "name": "Bob", "admin": true}`))
dec.DisallowUnknownFields() // reject unknown keys
err = dec.Decode(&u)
fmt.Println(err) // json: unknown field "admin"

// To reject trailing junk after the first object:
dec = json.NewDecoder(strings.NewReader(`{"id": 3} {"id": 4}`))
dec.Decode(&u)
if dec.More() {
	fmt.Println(errors.New("unexpected trailing data"))
}
```

#### Gotchas

- **Field-name matching is case-insensitive** on unmarshal (a JSON key matches any struct field name, regardless of case). This is a common surprise.
- A nil slice marshals as `null`, an empty slice as `[]`. This matters for APIs.
- Numbers decoded into `any` become `float64`, so large integers lose precision. Use `dec.UseNumber()` to get `json.Number` instead.
- `omitempty` does **not** omit empty structs; use `omitzero` (1.24) for that.
- An embedded type with a `MarshalJSON` method (e.g., embedded `time.Time`) changes how the outer struct marshals.

#### Streaming JSON

For newline-delimited JSON (NDJSON), decode in a loop:

```go
type Event struct {
	Type string `json:"type"`
	N    int    `json:"n"`
}

// A stream of JSON objects, one after another (JSON Lines, for example)
f := strings.NewReader(`{"type": "click", "n": 1}
{"type": "view", "n": 7}
{"type": "click", "n": 2}`)

dec := json.NewDecoder(f)
for dec.More() {
	var obj Event
	if err := dec.Decode(&obj); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", obj)
}
```

For writing JSON:

```go
items := []map[string]any{{"id": 1, "tags": []string{"a"}}, {"id": 2}}
enc := json.NewEncoder(os.Stdout)
enc.SetIndent("", "  ") // pretty-print
for _, item := range items {
	enc.Encode(item) // appends newline
}
```

#### Custom types

Implement `json.Marshaler` and `json.Unmarshaler` for custom encoding:

```go
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (u User) MarshalJSON() ([]byte, error) {
	// return custom JSON bytes
	return json.Marshal(map[string]any{
		"id":   u.ID,
		"name": strings.ToUpper(u.Name),
	})
}

func (u *User) UnmarshalJSON(b []byte) error {
	type Alias User // same fields, but none of User's methods: no infinite recursion
	aux := struct {
		*Alias
		Name string `json:"name"` // shadows Alias.Name so we can transform it
	}{Alias: (*Alias)(u)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	u.Name = strings.ToLower(aux.Name)
	return nil
}

func main() {
	out, _ := json.Marshal(User{ID: 7, Name: "ada"})
	fmt.Println(string(out)) // {"id":7,"name":"ADA"}

	var u User
	err := json.Unmarshal(out, &u)
	fmt.Println(u, err) // {7 ada} <nil>
}
```

### Go 1.24: os.Root

Go 1.24 adds `os.Root` to confine file operations to a single directory, preventing path traversal attacks when handling user-supplied paths:

```go
dir, _ := os.MkdirTemp("", "root-demo") // stands in for /var/app/data
defer os.RemoveAll(dir)

root, err := os.OpenRoot(dir)
if err != nil {
	log.Fatal(err)
}
defer root.Close()

f, err := root.Create("output.txt") // creates <dir>/output.txt
fmt.Println(err)                    // <nil>
f.Close()

_, err = root.Open("../../etc/passwd") // error: path escapes from parent
fmt.Println(err)
```

## Idioms & Best Practices

- **Accept `io.Reader`, return concrete types.** Functions should work against the interface, not a concrete type. This is the core of Go's composability.

- **Always check `sc.Err()` after a `bufio.Scanner` loop.** Forgetting this is a classic bug: the scanner silently stops at 64 KiB.

- **Raise the `bufio.Scanner` token limit only when necessary.** If you're handling unbounded lines, you probably want a different approach.

- **Streaming is better than whole-file for large or untrusted input.** Use `io.LimitReader` to protect against memory exhaustion.

- **Check Close errors on writers, not on readers.** Data may only hit the disk or network on Close or Flush, so `f.Close()` can fail.

- **Use a named error for Close in a deferred function.** Capture and check the Close error separately so Flush errors propagate correctly.

- **Disallow unknown JSON fields for configs and APIs.** Use `dec.DisallowUnknownFields()` to catch typos in JSON keys.

- **Compose readers and writers for pipelines.** Don't write data to intermediate buffers; use `io.TeeReader`, `io.MultiWriter`, and `io.Copy` to build efficient chains.

- **Remember that `io.Read` returns `n` and `err` together.** Process n bytes before checking the error.

- **Use `filepath.WalkDir` instead of `Walk`.** It's more efficient because `DirEntry` caches stat info.

## Common Pitfalls

- **Forgetting to check `sc.Err()` after a Scanner loop.** The scanner stops silently at 64 KiB.

- **Processing `io.EOF` as a failure.** `io.EOF` is not an error; it signals end-of-stream. Many operations return it together with data.

- **Buffered output lost on `os.Exit`.** `os.Exit` does not run deferred functions, so `defer w.Flush()` doesn't help in a CLI's main.

- **Reslicing a huge slice to keep only a part.** The whole array stays in memory. Use `slices.Clone` or copy to drop the parent.

- **Ignoring the error from `csv.Writer.Flush` or `w.Error()`.** Writes are buffered; errors surface on Flush or at the end.

- **Using `io.ReadAll` on untrusted input.** This can exhaust memory. Use `io.LimitReader` instead.

- **Not handling `n > 0` and `err != nil` together in a manual Read loop.** This is the `io.Reader` contract.

- **Assuming JSON keys are case-sensitive.** Unmarshal matches case-insensitively, which surprises many developers.

- **Using `omitempty` when you meant `omitzero`.** `omitempty` omits empty strings and empty slices, but not zero structs.

- **Keeping slices obtained from `sc.Bytes()`.** They reference the scanner's internal buffer, which is reused.

## Exercises

Complete the following exercises in the order listed. Each exercise has a test file that you must make pass by implementing the function or method described in the starter code.

### Exercise 1: rot13 Reader

**What to implement:** A custom `io.Reader` that applies ROT13 transformation to bytes flowing through it.

**What you'll learn:** How to wrap a reader, understand `io.Read` semantics (n and err together), and handle partial reads correctly.

**Test command:**
```bash
go test ./modules/05-io/exercises/rot13/
```

### Exercise 2: linecount

**What to implement:** A function that counts lines and reports the longest line, handling lines of any length without loading the entire line into memory.

**What you'll learn:** `bufio.Reader.ReadLine` for unbounded lines, the difference between the 64 KiB Scanner limit and streaming approaches, and how to track state across chunks.

**Test command:**
```bash
go test ./modules/05-io/exercises/linecount/
```

### Exercise 3: csvjson

**What to implement:** A function that converts CSV to newline-delimited JSON (NDJSON), streaming one row at a time.

**What you'll learn:** `encoding/csv` and `encoding/json` with streaming (Decoder/Encoder), header validation, and proper error handling and wrapping.

**Test command:**
```bash
go test ./modules/05-io/exercises/csvjson/
```

### Exercise 4: tail

**What to implement:** A function that returns the last N lines of a stream using a ring buffer, without loading the whole file.

**What you'll learn:** Ring buffers for fixed-size history, the token limit issue with Scanner, and how to manage memory under constraints.

**Test command:**
```bash
go test ./modules/05-io/exercises/tail/
```

### Exercise 5: config

**What to implement:** A JSON config loader with custom unmarshaling (Duration), validation, and proper error handling.

**What you'll learn:** Custom JSON types with `Unmarshaler`, validation with `errors.Join`, `DisallowUnknownFields`, file operations, and comprehensive error messages.

**Test command:**
```bash
go test ./modules/05-io/exercises/config/
```

## Project

See `project/README.md` for the **Log Analyzer** project: parse HTTP access logs, aggregate status codes and paths, compute p50/p95 latency, and output as text or JSON, all without loading the entire file into memory.

## Submitting for Review

When all exercise tests pass, ask your instructor (Claude) to review `modules/05-io/`. The instructor will check:

- Correct implementation of the core concepts (streaming, error handling, `io.Reader`/`io.Writer` patterns)
- No unnecessary allocations or buffering (logs should stream, not load)
- Proper error handling and wrapping
- Code cleanliness and idiomatic Go

## Further Reading

- **io package:** https://pkg.go.dev/io
- **bufio package:** https://pkg.go.dev/bufio
- **encoding/csv:** https://pkg.go.dev/encoding/csv
- **encoding/json:** https://pkg.go.dev/encoding/json
- **filepath.WalkDir:** https://pkg.go.dev/path/filepath#WalkDir
- **Go Blog – JSON and Go:** https://go.dev/blog/json
- **Go 1.24 release notes – os.Root:** https://go.dev/blog/osroot
- **100 Go Mistakes #46:** Using a filename as input instead of io.Reader
- **100 Go Mistakes #54:** Ignoring Close and Flush errors on writes
- **100 Go Mistakes #79:** Ignoring errors from Close and Write operations
