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
```

These abstractions let you write code that works with any source or sink: files, network connections, buffers, pipes, or custom implementations. The key insight is to write functions *against the interface*, not against a concrete type like `*os.File`.

**Why this matters:** A function that accepts `io.Reader` can be tested with a `strings.Reader`, can be used with a file, and can be composed with other readers. A function that accepts `*os.File` can only be used with files, and is much harder to test.

```go
// Good: works with any reader
func Count(r io.Reader) (int, error) { ... }

// Bad: only works with files, hard to test
func Count(f *os.File) (int, error) { ... }

// Usage
Count(os.Open("data.txt"))           // works
Count(strings.NewReader("test"))     // also works
Count(NewCustomReader(...))          // also works
```

### Read Semantics: `n` and `err` Together

One of the trickiest parts of `io.Reader` is that `Read` may return both `n > 0` **and** `err != nil` in the same call. This happens at EOF: the last chunk may be returned together with `io.EOF`. Always process the n bytes first, then look at the error.

```go
// Common mistake: ignoring n when err != nil
for {
    n, err := r.Read(p)
    if err != nil {
        return err // WRONG: you lose the n bytes that were read
    }
    process(p[:n])
}

// Correct
for {
    n, err := r.Read(p)
    if n > 0 {
        process(p[:n])
    }
    if err != nil {
        if err == io.EOF {
            break
        }
        return err
    }
}

// Or use helpers
all, err := io.ReadAll(r)           // handles n/err correctly
n, err := io.ReadFull(r, p)         // read exactly len(p) bytes or error
n, err := io.CopyN(w, r, 42)        // read/write exactly 42 bytes
```

### Composing Readers and Writers

The standard library provides composable building blocks:

```go
// io.Copy copies from src to dst until src returns io.EOF
io.Copy(dst io.Writer, src io.Reader) (int64, error)

// io.MultiReader concatenates several readers
multi := io.MultiReader(reader1, reader2, reader3)
io.Copy(os.Stdout, multi)

// io.MultiWriter broadcasts writes to multiple writers
multi := io.MultiWriter(file1, file2, os.Stdout)
fmt.Fprintf(multi, "goes to all three")

// io.TeeReader reads from src, writes to dst, and returns a reader
// that gives you the same bytes that were written to dst
tee := io.TeeReader(src, dst)
data, _ := io.ReadAll(tee)  // data == what was written to dst

// io.LimitReader returns a reader that stops after n bytes
limited := io.LimitReader(src, 1024)  // read at most 1024 bytes

// io.SectionReader reads from a reader at a given offset and length
section := io.NewSectionReader(f, offset, length)
```

These compose to build powerful pipelines with minimal copying:

```go
// Compress a file, write both the original and compressed to separate files
src, _ := os.Open("data.txt")
defer src.Close()

orig, _ := os.Create("data.txt.orig")
defer orig.Close()

gzipped, _ := os.Create("data.txt.gz")
defer gzipped.Close()

// Use TeeReader to observe the data flow, then compress
tee := io.TeeReader(src, orig)
gz := gzip.NewWriter(gzipped)
defer gz.Close()

io.Copy(gz, tee)  // reads from src, writes to both orig and gzipped
```

### Buffered I/O with bufio

For character-at-a-time or line-at-a-time reading, the `bufio` package provides buffering:

#### bufio.Scanner

`Scanner` reads lines or other tokens without holding the entire input in memory. It handles line ending conversion and raises an error if a line exceeds `bufio.MaxScanTokenSize` (64 KiB).

```go
sc := bufio.NewScanner(r)
for sc.Scan() {
    line := sc.Text()  // a string, without the newline
    // process line
}
if err := sc.Err(); err != nil {
    return err  // NEVER skip this check
}
```

The 64 KiB limit is intentional: it prevents memory exhaustion on malformed input. If you need longer lines, either:

1. Raise the limit with `sc.Buffer`:
   ```go
   sc := bufio.NewScanner(r)
   sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)  // max 10 MiB per line
   ```

2. Use `bufio.Reader.ReadLine` instead, which is more flexible:
   ```go
   br := bufio.NewReader(r)
   for {
       chunk, isPrefix, err := br.ReadLine()  // true means the line continues
       // ...
   }
   ```

3. Read in fixed-size chunks yourself and search with `bytes.IndexByte`.

#### bufio.Writer

A `Writer` buffers writes for efficiency. You must call `Flush` before closing to ensure all data is written.

```go
w := bufio.NewWriter(f)
fmt.Fprintf(w, "buffered output\n")
if err := w.Flush(); err != nil {
    return err
}
```

In a CLI, remember that `os.Exit` does not run defers, so buffered output is lost if you exit without flushing.

### File Operations and Error Handling

#### Whole-file I/O

For small or bounded files, `os.ReadFile` and `os.WriteFile` are simple and safe:

```go
data, err := os.ReadFile("config.json")
if err != nil {
    return err
}

err = os.WriteFile("output.txt", data, 0o644)
if err != nil {
    return err
}
```

#### Streaming and Cleanup

For large or untrusted input, stream with `io.Reader`/`io.Writer`. Always close files and check the error:

```go
// For reading: Close error is usually safe to ignore
f, err := os.Open("data.txt")
if err != nil {
    return err
}
defer f.Close()
// ... use f ...

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
```

For atomic replacements (preventing partial writes on crash), write to a temp file first, then rename:

```go
f, _ := ioutil.TempFile(dir, "tmp-")
// write to f
f.Close()
os.Rename(f.Name(), finalPath)
```

#### Protecting Against Resource Exhaustion

When accepting untrusted input, limit its size:

```go
// Bad: io.ReadAll on untrusted input can exhaust memory
data, _ := io.ReadAll(r)

// Good: limit to 1 MiB
limited := io.LimitReader(r, 1024*1024)
data, _ := io.ReadAll(limited)

// For HTTP request bodies, http.MaxBytesReader is built-in
r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
```

### Walking Directory Trees

The `filepath` package provides `WalkDir` for traversing directories without excessive allocations:

```go
err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
    if err != nil {
        return err  // permission denied, etc.
    }
    if d.IsDir() {
        return nil  // skip into directories
    }
    fmt.Println(path)
    return nil
})
```

Use `WalkDir`, not the older `Walk`: it's more efficient because `DirEntry` caches stat info.

### encoding/csv

The `csv` package reads and writes CSV with proper quoting and escape handling:

```go
import "encoding/csv"

// Reading
r := csv.NewReader(f)
r.FieldsPerRecord = 3  // enforce 3 fields per row, or -1 for variable
for {
    record, err := r.Read()  // []string, one per field
    if err == io.EOF {
        break
    }
    if err != nil {
        return err  // *csv.ParseError has Line and Column fields
    }
    process(record)
}

// Writing
w := csv.NewWriter(os.Stdout)
w.Write([]string{"name", "age"})
w.Write([]string{"Alice", "31"})
w.Flush()  // MUST flush before closing
if err := w.Error(); err != nil {
    return err
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
```

Fields without a tag use their struct name (case-sensitive). Unexported fields are always ignored.

#### Unmarshaling

`json.Unmarshal` is for data you already have as `[]byte`. `json.NewDecoder(r).Decode(&v)` is for streams:

```go
// Whole buffer
var u User
err := json.Unmarshal(data, &u)

// Stream (e.g., HTTP body, file)
dec := json.NewDecoder(f)
dec.DisallowUnknownFields()  // reject unknown keys
err := dec.Decode(&u)
// To reject trailing junk after the first object:
if dec.More() {
    return errors.New("unexpected trailing data")
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
dec := json.NewDecoder(f)
for dec.More() {
    var obj MyType
    if err := dec.Decode(&obj); err != nil {
        return err
    }
    process(obj)
}
```

For writing JSON:

```go
enc := json.NewEncoder(os.Stdout)
enc.SetIndent("", "  ")  // pretty-print
for _, item := range items {
    enc.Encode(item)  // appends newline
}
```

#### Custom types

Implement `json.Marshaler` and `json.Unmarshaler` for custom encoding:

```go
func (u User) MarshalJSON() ([]byte, error) {
    // return custom JSON bytes
    return json.Marshal(map[string]interface{}{
        "id": u.ID,
        "name": strings.ToUpper(u.Name),
    })
}

func (u *User) UnmarshalJSON(b []byte) error {
    type Alias User
    var aux struct {
        *Alias
        Name string `json:"name"` // uppercase before use
    }
    if err := json.Unmarshal(b, &aux); err != nil {
        return err
    }
    u.Name = strings.ToLower(aux.Name)
    return nil
}
```

### Go 1.24: os.Root

Go 1.24 adds `os.Root` to confine file operations to a single directory, preventing path traversal attacks when handling user-supplied paths:

```go
root, err := os.OpenRoot("/var/app/data")
defer root.Close()

f, err := root.Create("output.txt")  // creates /var/app/data/output.txt

f, err := root.Open("../../etc/passwd")  // error: "path escapes from parent"
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
