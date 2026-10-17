# Go Idioms and Common Mistakes: Author Notes (Go 1.24, stdlib only)

Research date: 2026-10-11. Each topic section stands on its own so module authors can copy it.
Conventions: GOOD/BAD snippets are short sketches, not full programs. "UNVERIFIED" means I did not confirm it in a primary source during this pass.
"100GM #n" means mistake number n at https://100go.co (Teiva Harsanyi, *100 Go Mistakes*).

## Version facts course authors should know (checked against release notes)

- **Go 1.20:** `errors.Join`, multiple `%w` in `fmt.Errorf`, `context.WithCancelCause`, `strings.Clone`/`bytes.Clone`, `go build -cover`.
- **Go 1.21:** built-ins `min`, `max` and `clear`; `slices`, `maps` and `cmp` packages; `log/slog`; `context.AfterFunc`/`WithoutCancel`/`WithTimeoutCause`/`WithDeadlineCause`; `sync.OnceFunc`/`OnceValue`/`OnceValues`; PGO is GA.
- **Go 1.22:**
  - Each loop iteration now gets its own loop variable, so the closure-capture bug is gone when go.mod says `go 1.22`+.
  - `for i := range 10` works.
  - ServeMux accepts method and wildcard patterns.
  - `math/rand/v2` added.
- **Go 1.23:**
  - range-over-func iterators (`iter.Seq`/`iter.Seq2`); `slices.Collect`, `slices.Sorted`, `maps.Keys`, `maps.Values`.
  - Unstopped Timers and Tickers can be garbage-collected, and timer channels are unbuffered (both need go.mod `go 1.23`+).
- **Go 1.24** (https://go.dev/doc/go1.24):
  - Testing: `testing.B.Loop`, `T.Context`/`B.Context`, `T.Chdir`.
  - `encoding/json` has an `omitzero` tag.
  - `strings`/`bytes` gain `Lines`, `SplitSeq`, `FieldsSeq`, `FieldsFuncSeq`, `SplitAfterSeq`.
  - New `os.Root` (filesystem access confined to one directory), `weak` package, `runtime.AddCleanup`.
  - Generic type aliases; Swiss-table maps.
  - `go.mod` `tool` directives.
  - New vet checks: the `tests` analyzer, and printf flags a non-constant format string (`fmt.Printf(s)`).
  - `crypto/rand.Read` never returns an error; new `crypto/rand.Text`.
- **NOT in 1.24 (watch out):**
  - `sync.WaitGroup.Go` arrived in Go 1.25.
  - `testing/synctest` is experimental in 1.24 (`GOEXPERIMENT=synctest`) and GA in 1.25. I'm fairly confident about the 1.25 status, but mark it UNVERIFIED.
  - errgroup lives in `golang.org/x/sync`, which is not stdlib.

---

## 1. Basics

### Naming
- Use MixedCaps or mixedCaps, never underscores. Unexported constants are `maxLength`, not `MAX_LENGTH` (CodeReviewComments, "Mixed Caps").
- Keep the case of an initialism consistent: `URL`/`url`, `ID`, `HTTPServer`, `userID`. Never `Url` or `Id`.
- Package names are short, lowercase, single words, and not plural. Avoid `util`, `common`, `helpers` and `base` (100GM #13; Google style).
- Don't repeat the package name in its identifiers: callers write `bytes.Buffer`, not `bytes.ByteBuffer`, and the constructor is `http.NewRequest` (Effective Go "Package names").
- Getters have no `Get` prefix: `Owner()` and `SetOwner()` (Effective Go "Getters").
- A one-method interface is named with an -er suffix: `Reader`, `Stringer`.
- Receiver names are 1–2 letters and the same on every method (`func (c *Client)`). Never `self` or `this`.
- Scope sets name length: short names for small scopes (`i`, `r`, `buf`), descriptive names for package-level identifiers.

### Packages and init
- Avoid `init()` for anything that can fail or that has side effects such as I/O or global flags. Prefer explicit construction (100GM #3; Uber "Avoid init()").
- Avoid mutable package globals and inject dependencies instead (Uber).
- Imports go in groups, stdlib first. Use blank imports only in `main` or tests (CodeReviewComments).

### Zero values
- Design types so the zero value is useful: `var mu sync.Mutex`, `var buf bytes.Buffer` and `var sb strings.Builder` all work without initialisation (Effective Go "Allocation with new").
- Don't take a pointer to a mutex; embed it or use a named field (Uber "Zero-value Mutexes are Valid").
- `var x T` is the clearest way to say "zero value". `x := T{}` is fine for composite literals.

### Slices
Length and capacity
- A slice header is (ptr, len, cap). `append` writes into the shared backing array when `len < cap`. Only when it runs out of capacity does it allocate a new array (go.dev/blog/slices-intro; 100GM #20).
- **Aliasing bug** (100GM #25):
  ```go
  a := make([]int, 3, 10)
  b := append(a[:2], 99) // overwrites a[2]!
  // Fix: full slice expression caps capacity so append must copy
  b := append(a[:2:2], 99)
  // Or: b := append(slices.Clone(a[:2]), 99)
  ```
- Treat `x = append(x, ...)` as the only form. Discarding the result is a bug, and `go vet` does not catch every case.
- `copy(dst, src)` copies `min(len(dst), len(src))` elements, so `dst := make([]T, 0, n); copy(dst, src)` copies nothing (100GM #24). Use `make([]T, len(src))`, or `slices.Clone`.

Preallocation (100GM #21; Uber "Specifying Container Capacity")
```go
// BAD: repeated growth
var out []string
for _, u := range users { out = append(out, u.Name) }
// GOOD
out := make([]string, 0, len(users))
```
Mixing `make([]T, n)` with `append` is a classic bug: it leaves n zero values at the front.

nil vs empty (CodeReviewComments "Declaring Empty Slices"; 100GM #22, #23)
- Prefer `var s []T` (nil) over `s := []T{}`.
- Check emptiness with `len(s) == 0`, never `s == nil`.
- The one place nil and empty differ: `encoding/json` marshals a nil slice as `null` and an empty slice as `[]`.

Other slice gotchas
- Range copies each element: `for _, v := range s { v.x = 1 }` has no effect. Use `s[i].x = 1` (100GM #30).
- Memory leaks:
  - Reslicing a huge array keeps the whole array alive. Clone the part you need.
  - When you shrink a `[]*T`, the elements past the new length are still referenced. Nil them out, or use `clear` (100GM #26).
- `slices.Delete` zeros the vacated tail elements (Go 1.22+).

### Maps
- **Iteration order is unspecified and intentionally randomized.** Never depend on it (100GM #33). For deterministic output, sort the keys: `for _, k := range slices.Sorted(maps.Keys(m))` (Go 1.23+).
- A nil map can be read and ranged over, and `len` works on it, but **writing to a nil map panics**. Initialise with `make` or a literal. A nil map field in a struct literal is a common bug.
- Preallocate with a hint: `make(map[K]V, n)` (100GM #27).
- Concurrent map writes, or a write concurrent with a read, cause a fatal runtime error that `recover` cannot catch. Guard the map with a `sync.Mutex`, or use `sync.Map` for the narrow cases its docs describe.
- Maps don't shrink after `delete`. A map that once held millions of entries keeps its buckets, so recreate it if that matters (100GM #28). It's UNVERIFIED whether the 1.24 Swiss-table implementation changes this; assume it still holds.
- `m[k].field = v` doesn't compile for struct values. Read, modify, write back, or store pointers.
- Use comma-ok to tell "missing" from "zero": `v, ok := m[k]`.
- Adding or deleting entries while ranging is allowed, but results are unspecified for entries added during iteration (spec).
- For sets, `map[K]struct{}` is the idiom. `map[K]bool` is fine when readability matters more.

### Strings, bytes, runes, UTF-8
Indexing and iteration (go.dev/blog/strings; 100GM #36–#41)
- A string is an immutable sequence of bytes. `len(s)` counts **bytes**.
- `s[i]` is a byte.
- `for i, r := range s` decodes UTF-8 and yields runes; `i` is the **byte offset**, not a rune index.
- Invalid UTF-8 decodes to `U+FFFD`.
- Count characters (code points) with `utf8.RuneCountInString(s)`.
- `[]rune(s)` allocates. Use it when you need random rune access.
- A rune is a code point, not a user-perceived character: "é" can be two runes. Grapheme handling is not in stdlib.

Trim, compare, convert
- `strings.TrimRight(s, "ab")` removes any trailing chars in the *set* "ab". `TrimSuffix` removes one exact suffix (100GM #38).
- Use `strings.EqualFold` for case-insensitive comparison, not `ToLower(a) == ToLower(b)`.
- Each `string` ↔ `[]byte` conversion generally copies. Stay in one domain: `bytes` mirrors most of `strings`, and many APIs such as `bufio` and `io` give you `[]byte` (100GM #40).
- Substrings share memory with the original string. Use `strings.Clone` (1.20) to drop a large parent (100GM #41).

### strings.Builder
```go
// BAD: O(n^2) copies
s := ""
for _, w := range words { s += w + "," }
// GOOD
var sb strings.Builder
sb.Grow(estimatedLen) // optional
for i, w := range words {
    if i > 0 { sb.WriteByte(',') }
    sb.WriteString(w)
}
return sb.String()
```
- `strings.Join` is simplest when you already have a slice.
- Don't copy a non-zero Builder; it panics.
- `fmt.Fprintf(&sb, ...)` works because `*Builder` is an `io.Writer`.

### Constants and iota
```go
type Weekday int
const (
    Sunday Weekday = iota // 0
    Monday
)
// Uber: start enums at 1 so the zero value means "unset" / invalid
const (
    _ Op = iota
    Add // 1
    Sub
)
// Or explicitly: OpUnknown Op = iota
const (
    _  = iota
    KB = 1 << (10 * iota)
    MB
)
```
- Untyped constants have arbitrary precision and get a type at the point of use.
- Give an enum a `String()` method. stdlib has no stringer generator, because `golang.org/x/tools/cmd/stringer` is not stdlib, so write it by hand with a switch or an array.
- Octal gotcha: `010 == 8`. Write `0o10` (100GM #17).
- Integer overflow wraps silently (100GM #18).

### Control flow basics
- Indent error flow: handle the error and return early, keeping the happy path unindented (CodeReviewComments; 100GM #2). Avoid `else` after `return`.
- Shadowing with `:=` in an inner scope is a classic bug (100GM #1):
  ```go
  var client *Client
  if tracing { client, err := newTraced() } // shadows outer client
  ```
- `break` inside `switch` or `select` within a `for` breaks only the switch. Use a labeled break (100GM #34).
- `defer` inside a loop runs at function exit, not at the end of each iteration, so file handles pile up. Move the loop body into a function (100GM #35).
- Deferred call arguments are evaluated when `defer` runs, not when the deferred call executes (100GM #47).

Sources: https://go.dev/doc/effective_go, https://go.dev/wiki/CodeReviewComments, https://google.github.io/styleguide/go/decisions, https://github.com/uber-go/guide/blob/master/style.md, https://go.dev/blog/slices-intro, https://go.dev/blog/strings, https://go.dev/blog/maps, https://100go.co

---

## 2. Errors

### Best practices
- Errors are values: return `error` as the **last** result and check it right away.
- Never discard an error with `_` without a comment explaining why (CodeReviewComments "Handle Errors").
- **Error strings** are lowercase with no trailing punctuation, because they get wrapped: `"open config: permission denied"` (CodeReviewComments "Error Strings").
- **Add context when wrapping.** Use `fmt.Errorf("load config %q: %w", path, err)` and put `%w` at the end (Google best practices).
  - Avoid filler like "failed to" and "error occurred": in a chain they repeat (Uber "Error Wrapping").
  - Don't repeat what the callee already says. `os.Open` already includes the path in its `*PathError`.
- **`%w` vs `%v`:** `%w` makes the wrapped error part of your API, because callers can now match it with `errors.Is`/`As`. Use `%v` to hide implementation details, for example at package or RPC boundaries (Google best practices "Adding information to errors"; 100GM #49).
- **Sentinel errors** are for expected, matchable conditions with no extra data. Name them `ErrXxx`; for an unexported sentinel use the `errXxx` form (Uber "Error Naming").
  ```go
  var ErrNotFound = errors.New("not found")
  if errors.Is(err, ErrNotFound) { ... }   // GOOD
  if err == ErrNotFound { ... }            // BAD once anything wraps it (100GM #51)
  ```
- **Typed errors** carry data. Name them `XxxError`, usually with a pointer receiver:
  ```go
  type ParseError struct{ Line int; Err error }
  func (e *ParseError) Error() string { return fmt.Sprintf("line %d: %v", e.Line, e.Err) }
  func (e *ParseError) Unwrap() error { return e.Err }

  var pe *ParseError
  if errors.As(err, &pe) { fmt.Println(pe.Line) }
  ```
  `errors.As` panics if its target isn't a non-nil pointer to a type that implements error or to an interface type. A type switch on `err.(type)` misses wrapped errors (100GM #50).
- Choosing a style (Uber "Error Types"):

  | Caller needs to match? | Message | Use |
  |---|---|---|
  | no | static | `errors.New` |
  | no | dynamic | `fmt.Errorf` |
  | yes | static | exported `var ErrX = errors.New(...)` |
  | yes | dynamic | custom error type |
- **`errors.Join(errs...)`** (1.20) combines independent errors, for example from validation or cleanup:
  - It returns nil if every argument is nil.
  - `errors.Is`/`As` look through all of them (`Unwrap() []error`).
  - `fmt.Errorf` with several `%w` verbs builds the same kind of tree.
- **Handle each error once:** either log it, or return it (wrapped), not both (Uber "Handle Errors Once"; 100GM #52; Dave Cheney's "Don't just check errors, handle them gracefully"). Logging and then returning produces duplicate log lines.
- Use custom `Is(target error) bool` methods only for unusual equivalence rules.
- **Don't use in-band errors.** Return `(T, error)` or `(T, bool)` instead of -1, "" or nil (CodeReviewComments "In-Band Errors").
- Handle errors from deferred `Close` on writers (see §6) (100GM #54).

### When to panic
- Panic only for programmer bugs and impossible states, such as violated invariants or an out-of-range index in your own logic, or for unrecoverable startup failures (Effective Go "Panic"; CodeReviewComments "Don't Panic"; 100GM #48).
- A `MustXxx` helper (like `regexp.MustCompile`) is fine for package-level initialisation with constant input.
- Library code shouldn't let panics escape its API. If it uses panic internally, it should `recover` and turn the panic into an error at the package boundary (Effective Go, `encoding/json` style).
- `log.Fatal` and `os.Exit` belong only in `main` (Uber "Exit in Main").

### Common mistakes
1. **The nil-typed-pointer-in-interface gotcha:**
   ```go
   func do() error {
       var e *MyErr // nil pointer
       return e     // NON-nil error: interface holds (type=*MyErr, value=nil)
   }
   // GOOD: return nil explicitly on success
   ```
   (Go FAQ "Why is my nil error value not equal to nil?"; 100GM #45)
2. Comparing with `==`, or type-asserting directly, after wrapping was introduced.
3. Capitalised or punctuated error strings, and `"Error: ..."` prefixes.
4. Wrapping at every layer with redundant text, like "failed to x: failed to y: failed to z".
5. Ignoring `Close`, `Flush`, `Write` and `rows.Err()` errors.
6. Using panic/recover as control flow.
7. Value-receiver vs pointer-receiver mismatch with `errors.As`. If `Error()` has a pointer receiver, the target must be `*T`.
8. Calling `err.Error()` and string-matching on the message.

Sources: https://go.dev/blog/go1.13-errors, https://go.dev/blog/errors-are-values, https://go.dev/doc/faq#nil_error, https://pkg.go.dev/errors, https://google.github.io/styleguide/go/best-practices#error-handling, https://github.com/uber-go/guide/blob/master/style.md#errors, https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully, https://100go.co/#error-management

---

## 3. CLI programs

### The testable-main pattern
```go
func main() {
    if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
        fmt.Fprintln(os.Stderr, "mytool:", err)
        os.Exit(1)
    }
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
    fs := flag.NewFlagSet("mytool", flag.ContinueOnError)
    fs.SetOutput(stderr)
    n := fs.Int("n", 10, "number of lines")
    if err := fs.Parse(args); err != nil {
        return err // includes flag.ErrHelp for -h
    }
    // ... use fs.Args(), write to stdout ...
    return nil
}
```
- Why this pattern:
  - Tests call `run` with a `bytes.Buffer` and fake args.
  - There is no global `flag.CommandLine` state.
  - `os.Exit` happens in exactly one place, so every defer inside `run` executes (Uber "Exit Once"; Mat Ryer's "How I write HTTP services after 13 years" uses the same `run` shape).
- **`os.Exit` doesn't run deferred functions** (pkg.go.dev/os#Exit), and `log.Fatal` calls `os.Exit(1)`. So `defer f.Close(); ...; log.Fatal(...)` never closes or flushes the file. Buffered output in an unflushed `bufio.Writer` is lost too.
- **Don't call `flag.Parse()` in `init()` or at package level.** Since Go 1.13, test flags are registered in `testing.Init`, called from the generated test main. Calling `flag.Parse` too early breaks `go test` with "flag provided but not defined: -test.…" (Go 1.13 release notes, testing section). Call it in `main`, or better, use a FlagSet inside `run`.
- Exit codes:
  - 0 means success, 1 a general error, and 2 a usage error. That's the convention `flag.ExitOnError` follows: it calls `os.Exit(2)`, and `os.Exit(0)` for `-h`/`-help` (pkg.go.dev/flag ErrorHandling docs).
  - Map `flag.ErrHelp` to exit 0 if you want `-h` to succeed.
- Messages:
  - Diagnostics go to **stderr**, data to **stdout**, so pipes stay clean.
  - Start error messages with the program name.
  - A CLI can print the error string as is, since it is lowercase by convention.
- Reading stdin:
  - Accept `io.Reader`.
  - Support `-` or "no file args" to mean stdin, the Unix filter convention.
  - Use `bufio.Scanner` for lines (but mind the 64 KiB token limit, §6) or `io.ReadAll` for small input.
  - Detect a TTY with `fi, _ := os.Stdin.Stat(); fi.Mode()&os.ModeCharDevice != 0`. This works but is a heuristic.
- Use `bufio.NewWriter(stdout)` for heavy output, and `defer`-check `Flush()`. Return the Flush error from `run`.
- Signals: `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM); defer stop()` gives `run` a cancellable context.
- Subcommands: one `flag.NewFlagSet` per subcommand, dispatched on `args[0]`.
- Flags:
  - `fs.Duration`, `fs.Func` (1.16) and `fs.TextVar` (1.19) let you avoid hand-parsing.
  - Implement `flag.Value` for repeatable flags.
  - The flag package stops parsing at the first non-flag argument, so `tool file -v` treats `-v` as a positional argument. Many beginners hit this.
- `fs.Usage` lets you customise help. `fs.PrintDefaults()` writes to the FlagSet's output.

### Testing a CLI
```go
func TestRun(t *testing.T) {
    var out, errb bytes.Buffer
    err := run([]string{"-n", "2"}, strings.NewReader("a\nb\nc\n"), &out, &errb)
    if err != nil { t.Fatal(err) }
    if got, want := out.String(), "a\nb\n"; got != want { t.Errorf("got %q, want %q", got, want) }
}
```
- For end-to-end tests:
  - `exec.Command(os.Args[0], "-test.run=TestHelperProcess")` is the classic stdlib self-exec trick.
  - Or build a binary in `TestMain`.
  - Or use `testscript`, which is not stdlib.

### Common mistakes
- Using global `flag.String` and `flag.Parse()` in `init`, which can't be tested and breaks `go test`.
- Calling `os.Exit`/`log.Fatal` deep in helpers, which skips defers and makes the code untestable.
- Printing errors to stdout.
- Not flushing a `bufio.Writer`.
- Ignoring the error from `scanner.Err()`.
- Exiting 0 on failure.

Sources: https://pkg.go.dev/flag, https://pkg.go.dev/os#Exit, https://go.dev/doc/go1.13#testing, https://github.com/uber-go/guide/blob/master/style.md#exit-in-main, https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/ (Mat Ryer; the `run` pattern)

---

## 4. Structs, methods, interfaces

### Receivers
Choosing a receiver type (CodeReviewComments "Receiver Type"; 100GM #42)
- **Pointer receiver** when the method mutates, when the struct contains a `sync.Mutex` or another no-copy field, when the struct is large, or when in doubt.
- **Value receiver** for small immutable types such as `time.Time`, maps, funcs, chans and small slices-as-values.
- Don't mix: once any method needs a pointer receiver, give all of them pointer receivers.

Method sets
- The method set of `T` contains the value-receiver methods only. `*T` has both.
- So `var _ Iface = T{}` fails if any method has a pointer receiver. "X does not implement Y (method has pointer receiver)" is the classic beginner compile error.
- Copying a struct that contains a Mutex is a bug. `go vet`'s copylocks check catches it, for example a value receiver on a type with a Mutex.

### Interfaces
- Keep them small: "The bigger the interface, the weaker the abstraction" (Rob Pike, Go Proverbs).
- **Define the interface where it's consumed, not in the implementing package.** Don't create interfaces "for mocking" before you need them (CodeReviewComments "Interfaces"; 100GM #5, #6).
- **Accept interfaces, return structs:** functions should return concrete types so you can add methods later without breaking callers (100GM #7). Returning `error` is the exception.
- Don't use `any` when a concrete type works, because `any` says nothing (100GM #8).
- Never use a pointer to an interface (Uber).
- **Compile-time assertion:**
  ```go
  var _ http.Handler = (*Server)(nil)
  var _ fmt.Stringer = Color(0)
  ```
- Type assertions: always use comma-ok, `s, ok := v.(string)`, unless a panic is truly intended (Uber).

### The nil interface vs nil pointer gotcha
```go
var p *T = nil
var i Iface = p
fmt.Println(i == nil) // false: (type=*T, value=nil)
```
An interface is nil only when both its type and its value are nil. A common source is a function returning a concrete nil pointer through an interface return type (see §2). Methods with pointer receivers can still be *called* on a nil `*T`.

### Embedding
- Embedding promotes the embedded type's methods. It's **not inheritance**: the receiver of a promoted method is the inner value, so there's no virtual dispatch back to the outer type.
- Avoid embedding in exported structs. It leaks the embedded type's API and couples you to it (Uber "Avoid Embedding Types in Public Structs"; 100GM #10).
- An embedded `sync.Mutex` exposes `Lock` and `Unlock` to users. Use a named unexported field: `mu sync.Mutex`.
- Interface embedding (`type ReadWriter interface { Reader; Writer }`) is idiomatic.
- Embedding an interface in a struct, for example to partially implement it in tests, panics at runtime if someone calls a method you didn't implement.

### Stringer
```go
func (c Color) String() string { ... }  // value receiver so both Color and *Color print
```
- **Infinite recursion trap** (Effective Go "Printing"): `func (m MyStr) String() string { return fmt.Sprintf("%s", m) }` recurses forever. Convert first: `fmt.Sprintf("%s", string(m))`.
- If `String` has a pointer receiver, printing a *value* won't call it.

### Constructors and options
- Name constructors `NewT` and return `*T` (or `T`).
- For many optional parameters, use a config struct or the functional options pattern (100GM #11; Dave Cheney's "Functional options for friendly APIs"). For a beginner course a plain config struct is often simpler.
- Avoid Java-style getters and setters for plain fields (100GM #4).
- Copy slices and maps at API boundaries when you store or return internal state (Uber "Copy Slices and Maps at Boundaries"; `slices.Clone`, `maps.Clone`).

### Common mistakes
- Mixed receivers.
- A value receiver on a method that mutates, so the mutation is lost.
- Copying structs that contain locks.
- Large "god" interfaces declared next to the implementation.
- Returning interfaces from constructors.
- Interfaces with a single implementation, created only for mocking.
- Embedding to "inherit".
- Comparing interfaces holding uncomparable dynamic types, which panics at runtime.

Sources: https://go.dev/doc/effective_go#interfaces_and_types, https://go.dev/wiki/CodeReviewComments#interfaces, https://go.dev/wiki/CodeReviewComments#receiver-type, https://go.dev/doc/faq#methods_on_values_or_pointers, https://go-proverbs.github.io/, https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance, https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis, https://100go.co

---

## 5. Testing

### Table-driven tests and subtests (go.dev/wiki/TableDrivenTests; 100GM #85)
```go
func TestParse(t *testing.T) {
    tests := []struct {
        name    string
        in      string
        want    int
        wantErr bool
    }{
        {"simple", "42", 42, false},
        {"empty", "", 0, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // safe w/o tt := tt since Go 1.22 loopvar semantics
            got, err := Parse(tt.in)
            if (err != nil) != tt.wantErr {
                t.Fatalf("Parse(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("Parse(%q) = %d, want %d", tt.in, got, tt.want)
            }
        })
    }
}
```
- Failure messages:
  - Use the `got X, want Y` style: name the function, the input, got, then want (CodeReviewComments "Useful Test Failures"; Google decisions "Got before want").
  - Prefer `t.Errorf`, which keeps going, to `t.Fatalf`, unless continuing makes no sense.
- Run one case with `go test -run 'TestParse/empty'`. Spaces in subtest names become `_`.
- Comparing values:
  - stdlib has no assert library, and Go style discourages assertion helpers (Google decisions "Assertion libraries").
  - Use `reflect.DeepEqual` or `slices.Equal`/`maps.Equal`.
  - `go-cmp` is not stdlib.

### Helpers and lifecycle
- `t.Helper()` makes failures report the caller's line.
- `t.Cleanup(func())` runs after the test and its subtests, LIFO. Prefer it to `defer` in helpers.
- `t.TempDir()` gives a fresh directory, removed automatically.
- `t.Setenv(k, v)` restores the variable automatically. It **panics if used with `t.Parallel()`** in the same test or its ancestors.
- `t.Chdir(dir)` (1.24) has the same parallel restriction (UNVERIFIED but documented as behaving like Setenv).
- `t.Context()` (1.24): canceled just before Cleanup functions run.
- `TestMain(m *testing.M)` is for package-level setup. Since 1.15 you don't need to call `os.Exit(m.Run())` yourself; returning is enough.

### Parallel tests
- `t.Parallel()` pauses the subtest until the parent's sequential part finishes.
- **Gotcha:** with parallel subtests, the parent's `defer` runs before the subtests finish. Use `t.Cleanup`, or wrap the subtests in a non-parallel group `t.Run`.
- Watch for shared mutable fixtures across parallel tests, and run with `-race`.

### testdata and golden files
- `go build` ignores directories named `testdata`. Tests run with the package directory as the working directory, so `os.ReadFile("testdata/in.txt")` works.
- Golden-file pattern:
  ```go
  var update = flag.Bool("update", false, "update golden files")
  // in test:
  golden := filepath.Join("testdata", tt.name+".golden")
  if *update { os.WriteFile(golden, got, 0o644) }
  want, _ := os.ReadFile(golden)
  ```
  Package-level flag vars in tests are fine; the testing package calls Parse. Run `go test -update`.
- `testing/fstest.MapFS` provides an in-memory filesystem. `testing/iotest` provides error-injecting readers.

### Black-box tests
- `package foo_test` in the same directory tests only the exported API. It can import `foo` and avoids import cycles.
- Use `package foo` (white-box) for internals.
- An `export_test.go` file can expose internals to the `_test` package.

### httptest (see also §10)
`httptest.NewRecorder()` for handler unit tests; `httptest.NewServer(h)` for client code against a real loopback server (`defer srv.Close()`).

### Examples (go.dev/blog/examples)
```go
func ExampleReverse() {
    fmt.Println(Reverse("abc"))
    // Output: cba
}
```
- Naming: `ExampleFoo`, `ExampleT_Method`, `Example_suffix` (lowercase suffix).
- An example runs as a test only when it has an `// Output:` comment. Use `// Unordered output:` for map iteration.
- Examples appear in godoc. The 1.24 vet `tests` analyzer flags examples that name nonexistent identifiers.

### Fuzzing (go.dev/doc/security/fuzz)
```go
func FuzzReverse(f *testing.F) {
    f.Add("hello")                  // seed corpus
    f.Fuzz(func(t *testing.T, s string) {
        r := Reverse(Reverse(s))
        if utf8.ValidString(s) && r != s { t.Errorf("roundtrip %q -> %q", s, r) }
    })
}
```
- `go test` with no fuzz flag runs only the seeds plus `testdata/fuzz/FuzzXxx/*` as regression tests.
- `go test -fuzz=FuzzReverse -fuzztime=30s` fuzzes. Failing inputs are written to `testdata/fuzz/FuzzReverse/`; commit them.
- Argument types are limited to string, []byte, int types, uint types, float types, bool and rune/byte.
- Only one fuzz target per `-fuzz` invocation, and the regexp must match exactly one.
- Good fuzz properties: round-trips, no panics, and agreement with an oracle.

### Benchmarks (Go 1.24: prefer b.Loop)
```go
func BenchmarkJoin(b *testing.B) {
    parts := makeParts(1000)    // setup, not timed
    b.ReportAllocs()
    for b.Loop() {
        _ = strings.Join(parts, ",")
    }
}
```
- How `b.Loop` behaves (go.dev/blog/testing-b-loop):
  - The function body runs **once** per `-count`, so setup isn't repeated.
  - The timer is reset at the first loop call and stopped at the end.
  - The compiler keeps arguments and results alive inside the loop, preventing dead-code elimination.
  - Only one `b.Loop` loop is allowed per benchmark, and every iteration should do identical work.
- Old style: `for i := 0; i < b.N; i++`, which needs `b.ResetTimer()` after setup and a package-level sink var to defeat the optimizer (100GM #89).
- Running:
  - `go test -bench=. -benchmem -count=10 -run='^$'` (`-run='^$'` skips the unit tests).
  - `-benchmem` and `b.ReportAllocs()` report B/op and allocs/op.
- Comparing runs: use `benchstat old.txt new.txt` (`golang.org/x/perf/cmd/benchstat`, a tool, not stdlib). Use `-count` of 10 or more so it can do statistics.
- Run sub-benchmarks with `b.Run("n=100", ...)`.

### Coverage
- `go test -cover`, or `-coverprofile=c.out` then `go tool cover -html=c.out` / `-func=c.out`.
- `-coverpkg=./...` counts cross-package coverage.
- `go build -cover` (1.20) is for integration coverage using `GOCOVERDIR`.
- Coverage is a guide, not a target.

### Other
- Never use `time.Sleep` for synchronisation in tests. Use channels, `sync.WaitGroup`, or injected clocks (100GM #86, #87). `testing/synctest` is experimental in 1.24.
- `go test -race` in CI (100GM #83). `-shuffle=on` finds order dependence (100GM #84).
- `go test -short` with `testing.Short()` skips slow tests. Build tags or env vars can categorise integration tests (100GM #82).
- `go vet` runs automatically during `go test` (a subset of checks).

Common mistakes
- Not calling `t.Helper()`.
- `t.Fatal` from a goroutine other than the test's. It's not allowed; use `t.Error` plus a return, or send the error back on a channel.
- Comparing errors by string.
- Parallel subtests sharing state.
- A `defer` in the parent of parallel subtests.
- Benchmarks whose results get optimized away (pre-`b.Loop`).
- Setup inside the timed loop.
- Forgetting `-run='^$'` when benchmarking.

Sources: https://pkg.go.dev/testing, https://go.dev/wiki/TableDrivenTests, https://go.dev/blog/subtests, https://go.dev/blog/examples, https://go.dev/doc/security/fuzz/, https://go.dev/doc/tutorial/fuzz, https://go.dev/blog/testing-b-loop, https://go.dev/doc/go1.24#testing, https://go.dev/blog/cover, https://google.github.io/styleguide/go/decisions#useful-test-failures, https://pkg.go.dev/net/http/httptest

---

## 6. I/O

### Composition
- `io.Reader` and `io.Writer` are the core abstractions. Write functions against them, not against `*os.File`, and let callers pass files, buffers, network connections or `strings.Reader` (100GM #46 "Using a filename as a function input").
- Composable pieces:
  - `io.Copy`, `io.TeeReader`, `io.MultiWriter`, `io.MultiReader`, `io.LimitReader`, `io.SectionReader`, `io.Pipe`.
  - `bufio.Reader`/`Writer`.
  - Compressors: `gzip.NewReader(r)`.
  - Hashes: `h := sha256.New(); io.Copy(h, f)`.
- `Read` semantics:
  - `Read` may return `n > 0` **together with** `err != nil`, including `io.EOF`. Process the n bytes first, then look at err.
  - Beginners usually get this wrong, so prefer `io.ReadFull`, `io.ReadAll` or `bufio`.

### bufio.Scanner
```go
sc := bufio.NewScanner(r)
for sc.Scan() {
    line := sc.Text()   // sc.Bytes() avoids alloc but is only valid until next Scan
}
if err := sc.Err(); err != nil { return err }  // never skip this
```
- **Token limit:** `bufio.MaxScanTokenSize = 64 * 1024`. A longer line makes `Scan` return false and `Err()` return `bufio.ErrTooLong`. Silently stopping is the classic bug when you forget to check `Err()`.
  - Raise the limit with `sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)`, called before the first Scan.
  - Or use `bufio.Reader.ReadString('\n')`/`ReadLine` for unbounded lines.
- Splitters: `bufio.ScanWords`, `ScanRunes`, `ScanBytes`, and custom `SplitFunc`s.
- `ScanLines` strips `\r\n` and `\n`, and it returns a final line that has no newline.
- `sc.Bytes()` aliases the internal buffer. Copy it before you store it.
- 1.24 alternative for in-memory strings: `for line := range strings.Lines(s)`, which *keeps* the newline.

### Close and its errors
```go
// Reads: ignoring Close error is acceptable
f, err := os.Open(p); if err != nil { return err }
defer f.Close()

// Writes: Close (and Flush) errors matter. Data may only hit disk on Close.
func write(p string, data []byte) (err error) {
    f, err := os.Create(p)
    if err != nil { return err }
    defer func() {
        if cerr := f.Close(); cerr != nil && err == nil { err = cerr }
    }()
    w := bufio.NewWriter(f)
    if _, err := w.Write(data); err != nil { return err }
    return w.Flush()
}
```
- Or use `errors.Join(err, f.Close())` in the deferred closure.
- Call `f.Sync()` before Close when durability matters.
- For atomic replacement, write a temp file in the same directory and then `os.Rename` it.
- Small whole-file writes can just use `os.WriteFile`.
- Related mistakes: 100GM #54 (defer errors) and #79 (not closing response bodies or files).

### Whole file vs streaming
- `os.ReadFile` and `os.WriteFile` are fine for small or bounded files.
- Stream with `bufio`/`io.Copy` for large or unbounded input, or for stdin and network data.
- `io.ReadAll` on untrusted input invites memory DoS. Wrap the reader with `io.LimitReader` (or `http.MaxBytesReader` for request bodies).
- `os.Root` (1.24) confines file operations to a directory, which helps against path traversal when handling user-supplied paths.

### encoding/csv
```go
r := csv.NewReader(f)
r.FieldsPerRecord = 3      // 0 = set by first record, -1 = variable
for {
    rec, err := r.Read()
    if err == io.EOF { break }
    if err != nil { return err } // *csv.ParseError has Line/Column
}
w := csv.NewWriter(out)
w.Write([]string{"a", "b"})
w.Flush()
if err := w.Error(); err != nil { return err }  // Write errors surface here
```
- `ReadAll` loads everything into memory. `ReuseRecord = true` reduces allocations, but then you must copy any record you keep.
- Set `r.Comma = '\t'` for TSV.
- `LazyQuotes` tolerates malformed quotes.
- A `csv.Writer` is buffered, so forgetting `Flush` loses output.

### encoding/json
```go
type User struct {
    ID      int       `json:"id"`
    Name    string    `json:"name"`
    Email   string    `json:"email,omitempty"`
    Created time.Time `json:"created,omitzero"` // 1.24: omitempty never omitted structs
    secret  string    // unexported: ignored
}
```
Unmarshal vs Decoder
- Use `json.Unmarshal` for a `[]byte` you already hold.
- Use `json.NewDecoder(r).Decode(&v)` for streams (HTTP bodies, files, NDJSON).
- `Decode` reads only the first value. To reject trailing junk, call Decode again and expect `io.EOF`, or check `dec.More()`.

Strictness
- `dec.DisallowUnknownFields()` rejects unknown keys, which is good for configs and APIs. There's no Unmarshal equivalent; wrap the bytes in a `bytes.Reader` and decode.

Gotchas (100GM #77)
- Field-name matching on Unmarshal is **case-insensitive**.
- Unexported fields are silently skipped.
- Numbers decoded into `any` become `float64`, so large int64 values lose precision. Use `dec.UseNumber()`.
- An embedded type with a `MarshalJSON` method, like an embedded `time.Time`, takes over the outer struct's marshaling.
- A nil slice marshals as `null`.
- A map marshals with sorted keys.
- `omitempty` doesn't omit zero structs; use `omitzero`.
- `time.Time` uses RFC 3339 and keeps the monotonic reading out of JSON, but `==` comparisons can still differ. Use `t.Equal`.
- Implementing `MarshalJSON` on a value receiver vs a pointer receiver changes when it gets called.

Encoding and validation
- Encode with `json.NewEncoder(w).Encode(v)`. It appends a newline, and `enc.SetIndent` controls formatting.
- `json.Valid` checks syntax.
- Custom types implement `json.Marshaler`/`Unmarshaler` or `encoding.TextMarshaler`. TextMarshaler also works for map keys.

Common mistakes
- Not checking `sc.Err()`.
- Hitting the 64 KiB limit.
- Ignoring `Close`/`Flush` errors on writes.
- `defer f.Close()` in a loop.
- Treating `n, err` from `Read` incorrectly.
- Keeping `sc.Bytes()` references.
- `io.ReadAll` on unbounded input.
- Forgetting `csv.Writer.Flush` or `w.Error()`.
- Misunderstanding the json tags `omitempty`, `string` and `-`.
- Exported vs unexported fields.

Sources: https://pkg.go.dev/io, https://pkg.go.dev/bufio#Scanner, https://pkg.go.dev/bufio#pkg-constants, https://pkg.go.dev/encoding/csv, https://pkg.go.dev/encoding/json, https://go.dev/blog/json, https://go.dev/doc/go1.24#encoding/json, https://pkg.go.dev/os#Root, https://100go.co/#standard-library

---

## 7. Generics

### When to use (go.dev/blog/when-generics, Ian Lance Taylor)
- "Write Go programs by writing code, not by defining types." Add type parameters later, when the need is concrete.
- "If you find yourself writing the exact same code multiple times, where the only difference between the copies is that the code uses different types, consider whether you can use a type parameter."
- **Use them for:**
  - functions over slices, maps and channels of any element type (`slices.Contains`, `maps.Keys`)
  - general-purpose data structures such as sets, trees and ring buffers
  - the case where every type shares an identical method implementation
- **Don't use them when:**
  - you only call methods on the value. Use an interface: `func Write(w io.Writer)`, not `func Write[W io.Writer](w W)`.
  - the implementation differs per type. Use interfaces.
  - the operation varies hugely per type. Use reflection, as `encoding/json` does.
- Prefer passing functions (`less func(a, b T) int`) to requiring methods on the constraint.
- Related: 100GM #9.

### Mechanics
```go
func Map[S ~[]E, E, R any](s S, f func(E) R) []R {
    out := make([]R, 0, len(s))
    for _, v := range s { out = append(out, f(v)) }
    return out
}

func Max[T cmp.Ordered](a, b T) T { if a > b { return a }; return b } // (builtin max exists since 1.21)

type Set[T comparable] map[T]struct{}
func (s Set[T]) Add(v T) { s[v] = struct{}{} }

type Number interface { ~int | ~int64 | ~float64 }
func Sum[T Number](xs ...T) T { var t T; for _, x := range xs { t += x }; return t }
```
Constraints
- `any` allows any type, but permits no operators except assignment and `==` **is not allowed**.
- `comparable` allows `==`/`!=` and use as a map key.
  - Since 1.20, interface types satisfy `comparable`, so `==` can panic at runtime when the dynamic types aren't comparable (go.dev/blog/comparable).
- `cmp.Ordered` (1.21) allows `< <= > >=`. Pair it with `cmp.Compare` and `cmp.Less`, which handle NaN.
- `~T` means "any type whose underlying type is T". Without `~`, `type Celsius float64` won't satisfy a `float64` constraint.
- `golang.org/x/exp/constraints` is not stdlib. Define your own unions or use `cmp.Ordered`.

Type inference
- Arguments usually infer type parameters: `Map(xs, strconv.Itoa)`.
- Inference fails when a type appears only in the result (`Zero[T]() T`). Then instantiate explicitly: `Zero[int]()`.
- The `S ~[]E` pattern, used by the `slices` package, preserves named slice types in results.

### Limitations that trip people up
- **Methods can't have their own type parameters.** Only the receiver's type parameters are available, so you can't write `func (l List[T]) Map[U any](...)`. Write a top-level function instead.
- You can't type-switch or type-assert directly on a type-parameter *value*; convert first with `switch any(v).(type)`. Needing that is often a sign generics are the wrong tool.
- You can't use a constraint interface with a type set (`int | float64`) as an ordinary variable type.
- No specialization and no default type arguments. No operator overloading: `Sum` over a custom type requires the union.
- Generic types need instantiation when used (`Set[string]{}`), though constructors can infer.
- Zero value of T: `var zero T`. `*new(T)` also works.
- Generic type aliases (`type A[T any] = ...`) are fully supported in 1.24.
- Performance:
  - The implementation uses GC-shape stenciling with dictionaries. Generic code called through interface-typed constraints may not be faster than interfaces, and can sometimes be slower.
  - Don't promise "generics are faster". PlanetScale's "Generics can make your Go code slower" (2022) is a secondary source; consider it UNVERIFIED for current versions.

Common mistakes
- Using generics where an interface suffices.
- Writing a generic wrapper over `slices`/`maps` functions that already exist: `slices.Index`, `slices.Sort`, `slices.Max`, `maps.Clone`, `slices.Collect(maps.Keys(m))`.
- Forgetting `~`.
- Assuming `comparable` excludes interfaces.
- Over-abstracted constraint hierarchies.

Sources: https://go.dev/blog/when-generics, https://go.dev/doc/tutorial/generics, https://go.dev/blog/intro-generics, https://go.dev/blog/comparable, https://go.dev/blog/deconstructing-type-parameters, https://pkg.go.dev/cmp, https://pkg.go.dev/slices, https://go.dev/ref/spec#Type_parameter_declarations

---

## 8. Concurrency

Ground rule (go.dev/ref/mem): "If you must read the rest of this document to understand the behavior of your program, you are being too clever. Don't be clever." A data race is any unsynchronized concurrent access where at least one access is a write. **Every goroutine needs a known way to exit** (CodeReviewComments "Goroutine Lifetimes"; 100GM #62).

### Channel rules (spec + Dave Cheney's "Channel Axioms")
| op | nil chan | closed chan | open chan |
|---|---|---|---|
| send | blocks forever | **panic** | blocks until received or buffer has room |
| receive | blocks forever | zero value immediately, `ok == false` | blocks until value |
| close | **panic** | **panic** | ok |

- **The owner (sender) closes; receivers never close.** With multiple senders, close from a coordinator after `wg.Wait()`.
- Closing is a broadcast: every receiver sees it. Use `close(done)` for cancellation and for "notification channels" (`chan struct{}`, 100GM #65).
- `for v := range ch` ends when ch is closed. Forgetting to close it leaks the receiver.
- Setting a channel variable to `nil` disables its `select` case. That's useful when merging channels that close at different times (100GM #66).
- `select` chooses **uniformly at random** among ready cases; there is no priority (100GM #64).
- Buffer size should be 0 or 1 unless you can justify the number (Uber "Channel Size is One or None"; 100GM #67). Buffers don't fix deadlocks; they hide them.
- Prefer synchronous APIs and let callers add the concurrency (CodeReviewComments "Synchronous Functions").

### Goroutine leaks
```go
// BAD: if ctx is canceled first, the goroutine blocks forever on send
func fetch(ctx context.Context) (string, error) {
    ch := make(chan string)
    go func() { ch <- slowCall() }()
    select {
    case s := <-ch: return s, nil
    case <-ctx.Done(): return "", ctx.Err()
    }
}
// FIX: ch := make(chan string, 1)  // sender never blocks
```
- Every blocking send or receive in a goroutine needs a matching `<-ctx.Done()`/`done` case or a guarantee that it completes.
- `time.After` in a loop:
  - Since 1.23, unstopped timers can be garbage-collected, so it no longer leaks until it fires (needs go.mod ≥ 1.23), but each call still allocates.
  - Prefer one `time.NewTimer` plus `Reset`, or a context timeout (100GM #76).
- Loop variable capture (`go func(){ use(i) }()`) is fixed since Go 1.22 for modules declaring go ≥ 1.22 (100GM #63). Older material is wrong for 1.24.

### sync.WaitGroup
```go
var wg sync.WaitGroup
for _, it := range items {
    wg.Add(1)              // BEFORE go, in the parent
    go func() {
        defer wg.Done()
        process(it)
    }()
}
wg.Wait()
```
- Calling `wg.Add(1)` *inside* the goroutine races with `Wait` (100GM #71).
- Don't copy a WaitGroup: pass `*sync.WaitGroup` or use closures (100GM #74; vet copylocks).
- `wg.Go(f)` exists only from **Go 1.25**. Don't use it in a 1.24 course.

### Mutex vs channels (100GM #57; Go wiki "MutexOrChannel")
- "Share memory by communicating" is guidance, not law.
- **Mutex** for protecting state such as caches, counters and maps.
- **Channels** for passing ownership of data, coordinating stages and signalling.
- Use mutexes this way:
  ```go
  type Counter struct {
      mu sync.Mutex
      m  map[string]int
  }
  func (c *Counter) Inc(k string) { c.mu.Lock(); defer c.mu.Unlock(); c.m[k]++ }
  ```
  - Keep critical sections small, and never do I/O or channel sends while holding a lock.
  - `RWMutex` only pays off for read-heavy workloads with longer critical sections (measure first).
- Mutex mistakes:
  - Returning an internal slice or map from under the lock and then using it outside, which is still a race. Copy it (100GM #70).
  - `append` on a shared slice from several goroutines races, even when capacity suffices (100GM #69).

### sync.Once, OnceFunc/OnceValue, atomic
- `sync.Once` for lazy initialisation. Since 1.21, `sync.OnceValue(func() T)` and `OnceValues` are cleaner. If the function panics, every call re-panics.
- `sync/atomic` typed values: `atomic.Int64`, `atomic.Bool`, `atomic.Pointer[T]` (1.19+). Prefer them to the `atomic.AddInt64(&x, 1)` functions because they can't be mixed with plain access by accident.
- Atomics are for single values. Updating two related fields atomically needs a mutex.
- `sync.Cond` is rarely needed; a channel close usually does the job (100GM #72).

### Race detector
- `go test -race ./...` and `go run -race`. It finds only races that actually execute, so tests need concurrency that exercises the code (100GM #58, #83).
- Expect roughly 5–10x CPU and memory overhead (per go.dev/doc/articles/race_detector).
- It requires cgo on most platforms (`CGO_ENABLED=1`). Treat the exact platform matrix as UNVERIFIED.
- A race can go silent and still exist. "Benign" races are not benign under the Go memory model.

### Patterns
Worker pool and fan-in
```go
func process(ctx context.Context, jobs []Job, workers int) []Result {
    in := make(chan Job)
    out := make(chan Result)
    var wg sync.WaitGroup
    for range workers {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := range in {
                select {
                case out <- do(j):
                case <-ctx.Done(): return
                }
            }
        }()
    }
    go func() {            // producer owns `in`
        defer close(in)
        for _, j := range jobs {
            select {
            case in <- j:
            case <-ctx.Done(): return
            }
        }
    }()
    go func() { wg.Wait(); close(out) }()  // closer owns `out`
    var res []Result
    for r := range out { res = append(res, r) }
    return res
}
```
Pipelines (go.dev/blog/pipelines)
- Each stage receives from an inbound channel and sends to an outbound one that it closes.
- On early exit, downstream signals upstream through a `done` channel or context so that blocked senders return.
- The blog uses `done chan struct{}`; modern code uses `ctx.Done()`.

Bounded concurrency with a semaphore
```go
sem := make(chan struct{}, limit)
for _, u := range urls {
    sem <- struct{}{}            // acquire (blocks when `limit` in flight)
    wg.Add(1)
    go func() {
        defer wg.Done()
        defer func() { <-sem }() // release
        fetch(u)
    }()
}
wg.Wait()
```
Acquire in the parent to bound the number of goroutines, not just the number of active ones.

errgroup-like, stdlib only (mimics `golang.org/x/sync/errgroup`)
```go
type Group struct {
    wg      sync.WaitGroup
    errOnce sync.Once
    err     error
    cancel  context.CancelCauseFunc
}
func WithContext(ctx context.Context) (*Group, context.Context) {
    ctx, cancel := context.WithCancelCause(ctx)
    return &Group{cancel: cancel}, ctx
}
func (g *Group) Go(f func() error) {
    g.wg.Add(1)
    go func() {
        defer g.wg.Done()
        if err := f(); err != nil {
            g.errOnce.Do(func() {
                g.err = err
                if g.cancel != nil { g.cancel(err) }
            })
        }
    }()
}
func (g *Group) Wait() error {
    g.wg.Wait()
    if g.cancel != nil { g.cancel(g.err) }
    return g.err
}
```
- Add a semaphore channel for a `SetLimit` equivalent.
- The real errgroup is worth mentioning (100GM #73), but it's not stdlib.
- Alternatively, collect all errors with a mutex-protected slice plus `errors.Join`.

### Concurrency isn't free
- Concurrency is not parallelism (Rob Pike's talk; 100GM #55).
- Goroutines have overhead. Tiny work items run slower in parallel, so measure (100GM #56).
- CPU-bound work: around `runtime.GOMAXPROCS(0)` workers. I/O-bound work can use more (100GM #59).
- In containers before Go 1.25, GOMAXPROCS doesn't respect cgroup CPU limits (100GM #100). Go 1.25 added cgroup-aware GOMAXPROCS; marked UNVERIFIED for exact version semantics, but it is not in 1.24.

### Common mistakes
- Leaked goroutines blocked on send or receive.
- Closing from the receiver side, closing twice, or sending after close.
- `wg.Add` inside the goroutine.
- Copying mutexes or WaitGroups.
- Unprotected map access, leading to "fatal error: concurrent map writes".
- `t.Fatal` from a spawned goroutine.
- Assuming `select` has priority.
- Sleep-based synchronisation.
- Unbounded goroutine spawning per item.
- Holding locks during I/O.
- Not running `-race`.

Sources: https://go.dev/ref/mem, https://go.dev/blog/pipelines, https://go.dev/blog/codelab-share, https://go.dev/doc/effective_go#concurrency, https://go.dev/wiki/MutexOrChannel, https://go.dev/doc/articles/race_detector, https://go.dev/talks/2012/concurrency.slide, https://dave.cheney.net/2014/03/19/channel-axioms, https://pkg.go.dev/sync, https://pkg.go.dev/sync/atomic, https://github.com/uber-go/guide/blob/master/style.md#channel-size-is-one-or-none, https://100go.co/#concurrency-foundations

---

## 9. Context

### Rules
- `ctx context.Context` is the **first parameter**, named `ctx` (CodeReviewComments "Contexts"; pkg.go.dev/context).
- **Don't store a Context in a struct.** Pass it to each method that needs it (go.dev/blog/context-and-structs).
  - Exception: types like `http.Request` that represent a single operation, or a struct that is itself request-scoped.
  - Never pass a nil ctx. If unsure, use `context.TODO()`.
- `context.Background()` belongs only at the top: `main`, init, tests, or the root of a server.
  - In tests, `t.Context()` (1.24) is the preferred choice.
- **Always call cancel**, usually with `defer cancel()`. Otherwise the timer and child resources leak until the parent is canceled; `go vet`'s lostcancel check catches this.
- **Cancellation propagates down only.** Canceling a parent cancels every child; children never affect parents.
  - Code must observe cancellation itself, via `select { case <-ctx.Done(): return ctx.Err() ... }`, or by passing ctx to APIs that do (`http.NewRequestWithContext`, `db.QueryContext`, `exec.CommandContext`).
  - A context doesn't magically stop CPU-bound loops. Check `ctx.Err()` periodically.

### Timeouts and deadlines
```go
ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()
req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
...
if errors.Is(err, context.DeadlineExceeded) { ... }
```
- `ctx.Err()` is `context.Canceled` or `context.DeadlineExceeded`, and the error returned by I/O often wraps it. Check with `errors.Is`.
- The *Cause variants: `WithCancelCause` (1.20), `WithTimeoutCause`/`WithDeadlineCause` (1.21), and `context.Cause(ctx)`.
- `context.WithoutCancel` (1.21) is for work that must outlive the request, like audit logging. `context.AfterFunc` (1.21) runs a function when ctx is done.
- Don't propagate a request context into background work that outlives the request. It gets canceled when the handler returns (100GM #61).

### Values
- Use values only for **request-scoped data that crosses API or process boundaries**: request IDs, trace spans, auth principals.
- **Never** pass optional parameters or dependencies (DB handles, loggers as config) through context (pkg.go.dev/context; 100GM #60).
- Keys must be an unexported type, so they can't collide:
  ```go
  type ctxKey struct{}
  func WithUser(ctx context.Context, u *User) context.Context { return context.WithValue(ctx, ctxKey{}, u) }
  func UserFrom(ctx context.Context) (*User, bool) { u, ok := ctx.Value(ctxKey{}).(*User); return u, ok }
  ```
  Never use string keys.
- Value lookup is a linked-list walk, which is O(depth). Keep values few.

### Common mistakes
- Context not first, or stored in a struct.
- Forgetting `cancel`.
- `context.Background()` deep in library code, which breaks cancellation.
- Using a request ctx for async work after the response.
- String keys.
- Putting configuration in values.
- Ignoring `ctx.Done()` in loops and select statements.
- Comparing `err == context.Canceled` without `errors.Is`.

Sources: https://pkg.go.dev/context, https://go.dev/blog/context, https://go.dev/blog/context-and-structs, https://go.dev/wiki/CodeReviewComments#contexts, https://google.github.io/styleguide/go/decisions#contexts, https://100go.co/#misunderstanding-go-contexts-60

---

## 10. HTTP (net/http)

### Go 1.22+ ServeMux patterns (go.dev/blog/routing-enhancements)
```go
mux := http.NewServeMux()
mux.HandleFunc("GET /posts/{id}", getPost)       // r.PathValue("id")
mux.HandleFunc("POST /posts", createPost)
mux.HandleFunc("GET /files/{path...}", serveFile) // rest of path
mux.HandleFunc("GET /{$}", home)                 // exactly "/", not a catch-all
mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS))) // FileServerFS 1.22
```
- Method behaviour:
  - A pattern for `GET` also matches `HEAD`.
  - A request whose path matches but whose method doesn't gets **405** with an `Allow` header.
- Precedence: **the most specific pattern wins**, so `/posts/latest` beats `/posts/{id}`. Two patterns where neither is more specific **panic at registration**.
- A trailing `/` still means a subtree, so `"/"` matches everything. Use `"/{$}"` for the root only. This is the most common beginner surprise.
- Host patterns: `"example.com/"`.
- `GODEBUG=httpmuxgo121=1` reverts to the old behaviour, and the go.mod version gates it.
- Don't register on `http.DefaultServeMux` (`http.HandleFunc`) in real code. Any imported package can add handlers to it, including `net/http/pprof`, which registers on `init`. Use your own mux.

### Handlers and middleware
```go
func logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        slog.Info("req", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
    })
}
handler := logging(recoverer(mux))
```
- Middleware is `func(http.Handler) http.Handler`.
- To capture the status code, wrap `ResponseWriter` in a struct. Implement `Unwrap() http.ResponseWriter` so `http.ResponseController` (1.20) still works.
- Handlers run concurrently, one goroutine per request. Shared state needs synchronisation.
- `net/http` recovers handler panics per connection and logs them. Your own recover middleware lets you return a 500.
- Use dependency-holding structs or closures, e.g. `func (s *Server) handleX() http.HandlerFunc`, instead of globals (Mat Ryer).

### Responses
```go
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)               // headers must be set BEFORE WriteHeader/Write
    if err := json.NewEncoder(w).Encode(v); err != nil {
        slog.Error("encode", "err", err) // too late to change status
    }
}
```
- Order matters: set headers, then `WriteHeader`, then `Write`. Changing headers after the first `Write` has no effect, and a second `WriteHeader` logs "superfluous response.WriteHeader call".
- **`return` after `http.Error(...)`** (100GM #80). `http.Error` doesn't stop the handler.
- For JSON you can't fully trust, encode into a `bytes.Buffer` first so you can still send a 500 if encoding fails.
- Request bodies:
  - Limit them: `r.Body = http.MaxBytesReader(w, r.Body, 1<<20)`.
  - Decode with a `json.Decoder` and `DisallowUnknownFields` where strictness is wanted.
  - The server closes `r.Body`; the handler doesn't need to.

### Server timeouts (100GM #81)
```go
srv := &http.Server{
    Addr:              ":8080",
    Handler:           handler,
    ReadHeaderTimeout: 5 * time.Second,  // mitigates Slowloris; gosec G112 flags missing
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      15 * time.Second,
    IdleTimeout:       60 * time.Second,
}
```
- `http.ListenAndServe(addr, h)` has **no timeouts**.
- Per-handler deadlines: `http.TimeoutHandler`, or `http.NewResponseController(w).SetWriteDeadline` (1.20).

### Graceful shutdown
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
errCh := make(chan error, 1)
go func() { errCh <- srv.ListenAndServe() }()
select {
case err := <-errCh:
    if !errors.Is(err, http.ErrServerClosed) { return err }
case <-ctx.Done():
}
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
return srv.Shutdown(shutdownCtx)
```
- `ListenAndServe` returns `ErrServerClosed` **immediately** when Shutdown is called. Make sure the program waits for `Shutdown` to return, or `main` exits and in-flight requests are killed (pkg.go.dev/net/http#Server.Shutdown).
- Shutdown doesn't wait for hijacked connections such as WebSockets. Use `RegisterOnShutdown` for those.
- Note: the shutdown context derives from `Background`, not from the already-canceled signal ctx.

### Clients
- **Never use `http.DefaultClient`/`http.Get` without a timeout.** The default has none, so a hung server hangs you forever (100GM #81). Either:
  ```go
  client := &http.Client{Timeout: 10 * time.Second}
  ```
  or use per-request contexts via `http.NewRequestWithContext`.
- Reuse one `http.Client` and its Transport; they're safe for concurrent use. Creating a client per request defeats connection pooling.
- Response bodies:
  - **Always `defer resp.Body.Close()`** after checking err. To allow connection reuse, read the body to EOF, e.g. `io.Copy(io.Discard, resp.Body)` (100GM #79).
  - A non-2xx status is **not** an error: check `resp.StatusCode`.

### Testing
```go
// Handler unit test
req := httptest.NewRequest(http.MethodGet, "/posts/42", nil)
rec := httptest.NewRecorder()
mux.ServeHTTP(rec, req)      // go through mux so PathValue is populated
res := rec.Result()
if res.StatusCode != http.StatusOK { ... }

// Client test against a real server
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ... }))
defer srv.Close()
resp, err := srv.Client().Get(srv.URL + "/x")
```
- If you call a handler directly without the mux, `r.PathValue` is empty. Use `req.SetPathValue("id", "42")` (1.22).
- `httptest.NewTLSServer` gives a TLS server.

### Common mistakes
- Missing `return` after an error response.
- Setting headers after `WriteHeader`.
- Using DefaultServeMux, or the default client or server without timeouts.
- Not closing response bodies.
- Treating 404/500 as `err != nil`.
- Unbounded request bodies.
- `"/"` catching everything unexpectedly.
- Shutdown handled incorrectly, with main exiting early.
- Data races in handlers.
- Using the request ctx in background goroutines.

Sources: https://go.dev/blog/routing-enhancements, https://pkg.go.dev/net/http#ServeMux, https://pkg.go.dev/net/http#Server.Shutdown, https://pkg.go.dev/net/http/httptest, https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/, https://blog.cloudflare.com/exposing-go-on-the-internet/, https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/, https://100go.co/#standard-library

---

## 11. Performance

### Measure first
- Profile and benchmark before optimising. "Premature optimization" applies, and so does "make it work, make it right, make it fast."
- Use `b.Loop` benchmarks with `-benchmem -count=10`, compare with `benchstat`, and change one thing at a time.
- The pprof workflow:
  - Benchmarks: `go test -bench=X -cpuprofile cpu.out -memprofile mem.out`, then `go tool pprof -http=:8080 cpu.out`.
  - Servers: `import _ "net/http/pprof"` on a **separate, non-public** mux or port, then `go tool pprof http://host/debug/pprof/profile?seconds=30`.
  - Other profiles: heap (`-sample_index=alloc_space` vs `inuse_space`), goroutine (finds leaks), block and mutex (need `runtime.SetBlockProfileRate`/`SetMutexProfileFraction`).
  - `go tool trace` for latency and scheduling.
- PGO (1.21+): put a CPU profile at `default.pgo` in the main package directory. Typical gains are 2–14% (per go.dev/doc/pgo; treat the number as UNVERIFIED).
- 100GM #98: use the diagnostics tooling.

### Escape analysis and allocations
- `go build -gcflags='-m'` (or `-m=2`) prints the compiler's escape decisions ("escapes to heap", "moved to heap", "inlining call").
- Typical causes of an escape:
  - returning a pointer to a local
  - storing it in an interface (`fmt.Println(x)` boxes x)
  - closures captured by goroutines
  - slices whose size isn't known at compile time (`make([]byte, n)` with variable n)
  - values too large for the stack
- Sharing down (passing pointers to callees) usually stays on the stack; sharing up (returning pointers) escapes (100GM #95).
- Ways to reduce allocations (100GM #96):
  - Preallocate slices and maps with known capacity.
  - Reuse buffers: `buf = buf[:0]`.
  - Use `strconv` instead of `fmt` (Uber: `strconv.Itoa` is about 2x faster than `fmt.Sprint`).
  - Use `strings.Builder` with `Grow`.
  - Prefer `AppendXxx`-style APIs: `strconv.AppendInt`, `time.AppendFormat`, `fmt.Appendf` (1.19).
  - Return values instead of pointers for small structs.
  - Avoid `interface{}` boxing in hot paths.
- Inlining (100GM #97): small leaf functions get inlined. `-gcflags=-m` shows "can inline". Mid-stack inlining is supported.

### Strings and []byte
- `s += x` in a loop is O(n²). Use `strings.Builder`, `strings.Join` or `bytes.Buffer` (100GM #39).
- Conversions the compiler optimises so they don't copy:
  - `m[string(b)]` map lookups
  - `string(b) == "lit"` comparisons
  - `for i, c := range []byte(s)`
  - `string(b)` in concatenation that doesn't escape
  Other conversions generally copy.
- Avoid repeated `[]byte("constant")` in a loop; hoist it out (Uber "Avoid repeated string-to-byte conversions").
- `unsafe.String`/`unsafe.StringData`/`unsafe.Slice` exist (1.20) but are out of scope for beginners and only safe when the bytes are never mutated.

### sync.Pool caveats
- Use it for reusing short-lived, frequently allocated objects, like buffers in encoders.
- **Not a cache:** items may be dropped at any GC, and since 1.13 they survive one GC through a victim cache.
- Store **pointers**. Putting a `[]byte` value in the pool allocates when it's boxed into `any` (staticcheck SA6002). Use `*[]byte` or `*bytes.Buffer`.
- **Reset** objects before `Put`, or after `Get`.
- Don't return huge buffers to the pool, because one large request pins memory. Cap the size you put back (golang/go issue 23199).
- Benchmark to prove it helps.

### Map vs slice for small sets
- For very small N, roughly up to tens of elements, a linear scan of a slice is often faster than hashing a map key, thanks to cache locality and no hashing. The crossover depends on key type and size; **benchmark it**. Any precise number is UNVERIFIED.
- `slices.Contains` on sorted or small data, `slices.BinarySearch` on sorted data.

### Memory layout
- Struct field alignment:
  - Ordering fields from largest to smallest alignment can shrink a struct: `struct{a bool; b int64; c bool}` is 24 bytes on 64-bit, while `{b int64; a, c bool}` is 16 (100GM #94).
  - The `fieldalignment` analyzer is in `golang.org/x/tools`, not in default `go vet`.
  - It matters only for structs allocated in large numbers. Readability comes first.
- CPU caches and false sharing (100GM #91, #92):
  - Prefer contiguous slices of values over slices of pointers.
  - Padding per-goroutine counters to a cache line (64 bytes on most x86-64, 128 bytes on some ARM) can avoid false sharing.

### Bounds-check elimination
- Check with `go build -gcflags='-d=ssa/check_bce/debug=1'`, which reports the remaining bounds checks.
- Hint the compiler with an early access: `_ = b[3]` before reading `b[0..3]` (`encoding/binary` uses this pattern).
- Range loops avoid the checks automatically.
- This is a micro-optimisation; teach it only as advanced material.

### GC tuning (100GM #99)
- `GOGC` (default 100) sets how much heap growth triggers a collection.
- `GOMEMLIMIT` (1.19) is a soft memory limit, useful in containers.
- Reducing allocations beats tuning.

### Common mistakes
- Optimising without profiling.
- Micro-benchmarks whose work the compiler eliminates (fixed by `b.Loop`).
- Comparing single benchmark runs without `-count`/benchstat.
- Using `fmt.Sprintf` for simple int-to-string conversion.
- Using `sync.Pool` with non-pointer values or without resetting.
- Exposing pprof publicly.
- Reading `alloc_space` vs `inuse_space` wrong.
- Assuming pointers are faster than values.
- Premature `unsafe`.

Sources: https://go.dev/doc/diagnostics, https://go.dev/blog/pprof, https://pkg.go.dev/runtime/pprof, https://pkg.go.dev/net/http/pprof, https://go.dev/doc/pgo, https://go.dev/doc/gc-guide, https://pkg.go.dev/sync#Pool, https://github.com/golang/go/issues/23199, https://pkg.go.dev/golang.org/x/perf/cmd/benchstat, https://go.dev/blog/testing-b-loop, https://github.com/uber-go/guide/blob/master/style.md#performance, https://100go.co/#optimizations, https://dave.cheney.net/high-performance-go-workshop/dotgo-paris.html

---

## Master source list
- Effective Go: https://go.dev/doc/effective_go. It's partly dated (pre-generics and pre-modules), so warn students.
- Code Review Comments: https://go.dev/wiki/CodeReviewComments
- Google Go Style Guide: https://google.github.io/styleguide/go/ (guide, decisions, best-practices)
- Uber Go Style Guide: https://github.com/uber-go/guide/blob/master/style.md
- 100 Go Mistakes: https://100go.co
- Dave Cheney: https://dave.cheney.net (Practical Go: https://dave.cheney.net/practical-go/presentations/qcon-china.html)
- Go Memory Model: https://go.dev/ref/mem
- Go FAQ: https://go.dev/doc/faq
- Release notes: https://go.dev/doc/go1.22, https://go.dev/doc/go1.23, https://go.dev/doc/go1.24
- Blogs:
  - errors: https://go.dev/blog/go1.13-errors
  - context: https://go.dev/blog/context
  - pipelines: https://go.dev/blog/pipelines
  - reflection: https://go.dev/blog/laws-of-reflection
  - slices: https://go.dev/blog/slices-intro
  - strings: https://go.dev/blog/strings
  - maps: https://go.dev/blog/maps
  - profiling: https://go.dev/blog/pprof
  - generics: https://go.dev/blog/when-generics
  - routing: https://go.dev/blog/routing-enhancements
  - benchmarks: https://go.dev/blog/testing-b-loop
  - loop variables: https://go.dev/blog/loopvar-preview

Laws of reflection, briefly, for any module that touches `reflect`:
1. Reflection goes from an interface value to a reflection object.
2. It goes from a reflection object back to an interface value.
3. To modify a reflection object, the value must be settable (`reflect.ValueOf(&x).Elem()`).

Advice for a course: avoid reflect except to explain how `encoding/json` works.
