# Module 02: Errors as Values

## Goals

After this module, you'll be able to:

- Understand error handling in Go: why errors are *values*, not exceptions.
- Use sentinel errors to represent expected failure modes.
- Create custom error types that carry extra data (line numbers, fields, etc.).
- Wrap errors with context using `fmt.Errorf("%w", err)`.
- Unwrap and inspect errors with `errors.Is` and `errors.As`.
- Combine multiple independent errors with `errors.Join`.
- Decide when `panic` and `recover` are appropriate.
- Follow error string conventions: lowercase, no punctuation, no "Error: " prefix.
- Handle errors once: either log or return, not both.
- Write deferred cleanup code that doesn't lose Close or Flush errors.

## Concepts

### The Philosophy: Errors as Values, Not Exceptions

In Go, an error is a value that implements the `error` interface—just one method:

```go
type error interface {
    Error() string
}

// The built-in error has exactly this method set, so any error satisfies it:
var e error = errors.New("boom")
fmt.Println(e.Error()) // boom
```

Errors are not exceptions; they flow through the program like any other value, subject to the same control flow rules as everything else. The convention is to return an error as the *last* result:

```go
func ReadFile(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("empty path") // failure: zero value + error
	}
	data := []byte("contents of " + path)
	return data, nil // success: value + nil error
}

data, err := ReadFile("notes.txt")
fmt.Printf("%s %v\n", data, err) // contents of notes.txt <nil>
_, err = ReadFile("")
fmt.Println(err) // empty path
```

This differs fundamentally from checked exceptions (Java) or try/catch (Python/JavaScript). Go's approach is explicit and visible at the call site. Every error *can* be checked because it's in the function signature. This makes error handling intentional and testable: you can reason about what paths the code will take because errors don't automatically propagate.

**Why this matters:**
- **Visible by default:** A function's signature tells you what errors it returns. In exception-based languages, you must read the documentation or implementation to know.
- **Composable:** You can handle errors selectively: ignore them (rarely), wrap them, transform them, or handle them gracefully. There's no hidden unwinding.
- **Testable:** Because errors are values, you can test error handling paths by constructing specific error values and checking for them.
- **Lightweight:** Errors are just values; creating and checking them is cheap. No stack unwinding or exception handling machinery.

**Key insight:** Errors are part of the API contract. A caller sees what can go wrong before calling the function. This encourages thinking about failure modes upfront.

### Sentinel Errors

For expected, known failure modes—no extra information needed—use *sentinel errors*. These are package-level error values, typically created with `errors.New`:

```go
var (
    ErrNotFound  = errors.New("not found")
    ErrWrongType = errors.New("wrong type")
)

err := fmt.Errorf("load user 7: %w", ErrNotFound)
fmt.Println(err)                                                     // load user 7: not found
fmt.Println(errors.Is(err, ErrNotFound), errors.Is(err, ErrWrongType)) // true false
```

Name them `ErrXxx` (exported) or `errXxx` (unexported). Callers match them with `errors.Is`:

```go
var ErrNotFound = errors.New("not found")

func lookup(id int) (string, error) {
	if id != 1 {
		return "", fmt.Errorf("lookup %d: %w", id, ErrNotFound)
	}
	return "Ada", nil
}

data, err := lookup(42)
if errors.Is(err, ErrNotFound) {
	// Handle the specific case, e.g., return a default value
	data = "guest"
}
fmt.Println(data) // guest
```

**Why sentinels?** They represent a specific condition that a caller might handle differently. For example:
- A network timeout might be retried.
- A "not found" error might return a default value.
- A validation error might be shown to a user.
- An I/O error might be logged and the program exit.

By using named sentinels, callers can *match* the specific condition without parsing the error message.

**Why not `==`?** Once an error is wrapped (more below), direct comparison fails:

```go
var ErrNotFound = errors.New("not found")

func someFunc() error { return fmt.Errorf("load item 7: %w", ErrNotFound) }

err := someFunc()

// Wrong: comparison fails if the error is wrapped
fmt.Println(err == ErrNotFound) // false, even though the underlying error IS ErrNotFound

// Right: errors.Is looks through wrapping
fmt.Println(errors.Is(err, ErrNotFound)) // true
```

`errors.Is` searches through the error chain looking for a match, so it works regardless of how many layers of wrapping occur.

### Wrapping and Context

When an error flows through layers of code, each layer can add context. Use `fmt.Errorf` with `%w`:

```go
func loadConfig(path string) error {
	_, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("load config %q: %w", path, err)
	}
	return nil
}

err := loadConfig("/no/such/config.json")
fmt.Println(err)                            // load config "/no/such/config.json": open /no/such/config.json: no such file or directory
fmt.Println(errors.Is(err, fs.ErrNotExist)) // true: still matchable through the wrap
```

This preserves the error chain: `errors.Is(err, ErrNotFound)` works even after wrapping. The last `%w` in a wrapped error becomes its "cause" (the `.Unwrap()` value).

**Why wrap?** Errors propagate up the call stack, losing context about where they came from. A caller who sees "not found" without knowing which file wasn't found, or which user wasn't found, can't reason about the failure. Wrapping adds that context:

```go
var ErrNoUser = errors.New("no such user")

type Store map[string]string

func (s Store) Get(id string) (string, error) {
	u, ok := s[id]
	if !ok {
		return "", ErrNoUser
	}
	return u, nil
}

type UserService struct{ store Store }

// Low level, unwrapped
func getUserBare(store Store, id string) error {
	_, err := store.Get(id)
	if err != nil {
		return err // caller doesn't know what operation failed or what ID
	}
	return nil
}

// Better: wrap with context
func getUser(store Store, id string) error {
	_, err := store.Get(id)
	if err != nil {
		return fmt.Errorf("get user %q: %w", id, err) // caller knows it's about a user and which ID
	}
	return nil
}

// Higher level wraps again
func (svc *UserService) GetUser(id string) error {
	err := getUser(svc.store, id)
	if err != nil {
		return fmt.Errorf("load user profile: %w", err) // now we know it's part of profile loading
	}
	return nil
}

svc := &UserService{store: Store{"u1": "Ada"}}
fmt.Println(getUserBare(svc.store, "u9")) // no such user
fmt.Println(svc.GetUser("u9"))            // load user profile: get user "u9": no such user
```

The error message chain becomes: "load user profile: get user \"123\": not found". Each layer adds information without hiding the underlying cause.

**Bad:**
```go
var ErrMissing = errors.New("missing")
err := fmt.Errorf("failed to load: %v", ErrMissing) // %v hides the cause from errors.Is
fmt.Println(err, errors.Is(err, ErrMissing))         // failed to load: missing false
```

When `%v` is used instead of `%w`, the error is converted to a string and loses its structure. Callers can't match the underlying sentinel error anymore.

**Good:**
```go
var ErrMissing = errors.New("missing")
err := fmt.Errorf("load config: %w", ErrMissing) // the cause stays matchable
fmt.Println(err, errors.Is(err, ErrMissing))      // load config: missing true
```

**Wrapping guidelines:**
- Wrap when you add meaningful context (operation, argument values, or location in code).
- Don't wrap at *every* layer; it creates noise. Skip layers that don't add information.
- Always use `%w`, never `%v` or `%s`.
- Put `%w` last in the format string; it's the most important part.

**Convention:** Error strings are lowercase, no period, and fit into a sentence: "open /etc/hosts: permission denied" not "Open: Permission Denied" or "Error: ...". The wrapped error will add more detail, so keep your context message concise.

### Custom Error Types

When an error carries extra data (line numbers, field names, a structured cause), define a struct with an `Error()` method and usually an `Unwrap()` method:

```go
type ParseError struct {
    Line int
    Msg  string
    Err  error  // optional: underlying cause, usually a sentinel
}

func (e *ParseError) Error() string {
    return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

func (e *ParseError) Unwrap() error {
    return e.Err
}

err := fmt.Errorf("load config: %w", &ParseError{Line: 3, Msg: "unexpected '}'", Err: io.ErrUnexpectedEOF})
fmt.Println(err)                                // load config: line 3: unexpected '}'
fmt.Println(errors.Is(err, io.ErrUnexpectedEOF)) // true: errors.Is follows Unwrap
```

**When to use custom types:**
- The error carries structured data that callers will need (line number, field name, amount, etc.).
- The error represents a high-level concern that spans multiple low-level failures (e.g., "validation failed" wraps multiple field errors).
- You need to provide extra methods (like `Fields()` to list affected fields).

Callers use `errors.As` to extract it:

```go
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

err := fmt.Errorf("read config: %w", &ParseError{Line: 3, Msg: "missing ="})

var pe *ParseError
if errors.As(err, &pe) {
	fmt.Printf("Problem at line %d\n", pe.Line) // Problem at line 3
}
```

The `errors.As` function walks the error chain looking for a value of the target type. If found, it assigns it to the pointer and returns true.

**Pointer receivers:** Custom error types usually have pointer receivers (`*ParseError`) so the type can be used with `errors.As`. The rule: `errors.As` requires a non-nil pointer to the target type.

**Example: Multiple validation errors with structure:**

```go
var ErrRequired = errors.New("required")

type FieldError struct {
	Field string
	Err   error // a sentinel like ErrRequired or ErrInvalidFormat
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("field %q: %v", e.Field, e.Err)
}

func (e *FieldError) Unwrap() error {
	return e.Err
}

err := fmt.Errorf("signup: %w", &FieldError{Field: "email", Err: ErrRequired})

// Callers can extract structured data:
var fe *FieldError
if errors.As(err, &fe) {
	// e.g. highlight the invalid field in a form
	fmt.Println("bad field:", fe.Field) // bad field: email
}

// Or match the underlying sentinel:
if errors.Is(err, ErrRequired) {
	// Handle all "required" failures
	fmt.Println("something required is missing")
}
```

**Error chains with custom types:** A custom error type can wrap another custom type, creating a chain:

```go
// Database layer returns an error with the table name
type TableError struct {
	Table string
	Err   error
}

func (e *TableError) Error() string { return e.Table + ": " + e.Err.Error() }
func (e *TableError) Unwrap() error { return e.Err }

// Application layer wraps that
type AppError struct {
	Operation string
	Err       error
}

func (e *AppError) Error() string { return e.Operation + ": " + e.Err.Error() }
func (e *AppError) Unwrap() error { return e.Err }

err := error(&AppError{"save order", &TableError{"orders", errors.New("disk full")}})

// A caller can extract either level:
var ae *AppError
if errors.As(err, &ae) {
	fmt.Println("operation:", ae.Operation) // operation: save order
}

var te *TableError
if errors.As(err, &te) {
	fmt.Println("table:", te.Table) // table: orders
}
```

### errors.Join (Go 1.20+)

When multiple independent errors occur—validation of several fields, cleanup failures—combine them with `errors.Join`:

```go
var (
	ErrRequired = errors.New("required")
	ErrInvalid  = errors.New("invalid")
)

type FieldError struct {
	Field string
	Err   error
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Err.Error() }
func (e *FieldError) Unwrap() error { return e.Err }

func validate(name string, age int) error {
	var errs []error
	if name == "" {
		errs = append(errs, &FieldError{"name", ErrRequired})
	}
	if age < 0 {
		errs = append(errs, &FieldError{"age", ErrInvalid})
	}
	return errors.Join(errs...) // returns nil if errs is empty
}

err := validate("", -1)
fmt.Println(err)                                                // name: required, then age: invalid (one per line)
fmt.Println(errors.Is(err, ErrRequired), errors.Is(err, ErrInvalid)) // true true
fmt.Println(validate("Ada", 36))                                // <nil>
```

**When to use `errors.Join`:**
- Multiple independent failures occur, and you want to report all of them, not just the first.
- Form validation: the user should see all invalid fields at once, not be forced to fix them one at a time.
- Cleanup failures: a deferred Close might fail after an earlier Write also failed.
- Concurrent operations: multiple goroutines fail independently.

**Key properties:**
- `errors.Join(nil, nil, nil)` returns `nil`. This lets you safely append conditional errors to a slice.
- `errors.Is` and `errors.As` work through a joined tree: they check every branch. So `errors.Is(joined, ErrRequired)` returns true if *any* joined error is or wraps `ErrRequired`.
- The Error() message includes all joined messages: `"field \"name\": required\nfield \"age\": invalid"`.

**Example: Joining write and close errors:**

```go
func SaveFile(path string, data []byte) error {
    f, err := os.Create(path)
    if err != nil {
        return err  // file creation failed; nothing to clean up
    }

    var writeErr error
    if _, err := f.Write(data); err != nil {
        writeErr = fmt.Errorf("write: %w", err)
    }

    closeErr := f.Close()
    if closeErr != nil {
        closeErr = fmt.Errorf("close: %w", closeErr)
    }

    return errors.Join(writeErr, closeErr)  // both are reported
}

fmt.Println(SaveFile(filepath.Join(os.TempDir(), "save-demo.txt"), []byte("hi"))) // <nil>
fmt.Println(SaveFile("/no/such/dir/file.txt", []byte("hi")))                      // open /no/such/dir/file.txt: no such file or directory
```

Both errors are visible to the caller, who can use `errors.Is` to check for either.

### Panic vs Errors

**Use errors for expected failures:** file not found, validation failure, network timeout, insufficient funds, invalid user input. Return `(T, error)`. The caller can handle it.

**Use panic for programmer bugs and impossible states:**
- Out-of-range index in your own code (not user input).
- Nil pointer dereference when the spec says it's always non-nil.
- Violated invariants (internal consistency checks).
- Unrecoverable startup failures (can't initialize the runtime).

The key distinction: if the error is something the *caller* might have caused (bad input, missing file, network issue), return an error. If it's a bug in the *program* (programmer error), panic.

```go
// OK: the caller provided a bad argument; they can handle it
func Open(path string) (*os.File, error) {
	if path == "" {
		return nil, errors.New("path is required")
	}
	return os.Open(path)
}

type Stack struct{ items []int }

func (s *Stack) Len() int { return len(s.items) }

// OK: this violates a package invariant (programmer bug)
func (s *Stack) Pop() int {
	if s.Len() == 0 {
		panic("Pop on empty stack") // programmer error; caller didn't check Len()
	}
	v := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return v
}

// NOT OK: panicking on user input
func ParseInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err) // NO! User provided bad input, return an error
	}
	return n, nil
}

_, err := Open("")
fmt.Println(err) // path is required
s := &Stack{items: []int{1}}
fmt.Println(s.Pop()) // 1
n, _ := ParseInt("42")
fmt.Println(n) // 42. Now try ParseInt("forty-two") or another s.Pop()
```

**MustXxx functions:** The pattern `MustXxx` (like `regexp.MustCompile`) is fine for initialization with constant, known-good input:

```go
// Initialized once at package level, with known-good input
var emailRegex = regexp.MustCompile(`^[^@]+@[^@]+$`)

// If the regex were wrong, it's a programmer error, so panicking at startup is appropriate.

fmt.Println(emailRegex.MatchString("ada@example.com"), emailRegex.MatchString("not an email")) // true false
```

But don't use `MustXxx` for user-provided data.

### Recover: One Legitimate Use

Library code that doesn't control its callbacks (HTTP handlers, plugin hosts, job runners) may use `recover` at a boundary to prevent panics from crashing the application:

```go
func ServeRequest(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if recovered := recover(); recovered != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Internal error: %v", recovered)
			// Log it for debugging
			log.Printf("panic in handler: %v", recovered)
		}
	}()
	handleLogic(r) // might panic, but we'll recover
}

func handleLogic(r *http.Request) {
	var counts map[string]int
	counts[r.URL.Path]++ // bug: nil map
}

// Call the handler directly, the way a test would (module 08 covers httptest).
rec := httptest.NewRecorder()
ServeRequest(rec, httptest.NewRequest("GET", "/boom", nil))
fmt.Println(rec.Code, rec.Body.String()) // 500 Internal error: assignment to entry in nil map
```

This is appropriate at *boundaries*—places where untrusted code runs:
- HTTP request handlers.
- Job queue processors executing user-submitted tasks.
- Plugin or callback hosts.
- RPC servers or message dispatchers.

The pattern: catch the panic, log it, and return a safe error response to the caller. The panic doesn't crash the server; the handler fails gracefully.

**Why only at boundaries?** 
- If *your* code panicked (not a callback you called), recovering masks a bug. You should fix it, not hide it.
- Recovering deep in libraries breaks the call stack contract. The caller expects either a return value or a crash, not a secret recovery.
- It makes the code harder to debug: a panic message disappears, and you're left with an error you need to correlate back to the panic location.

**Outside these boundaries, recover usually indicates a design problem.** Don't use it as control flow:

```go
var badState = true

// Bad: using recover to implement try/catch
func doSomethingPanicky() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("caught:", r) // handle the "error"
		}
	}()

	// Panicking instead of returning errors
	if badState {
		panic("something went wrong") // NO!
	}
}

// Good: return an error normally
func doSomething() error {
	if badState {
		return errors.New("something went wrong")
	}
	return nil
}

doSomethingPanicky()
if err := doSomething(); err != nil {
	fmt.Println("error:", err)
}
```

### Handle Errors Once

If an error is important enough to log, don't also return it—the caller will log it again, and you'll see duplicate messages in logs. Log *or* return, not both (Uber style guide):

```go
func loadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("failed to open: %v", err) // Bad: logs once...
		return err
	}
	return f.Close()
}

// ...and the caller logs it again
if err := loadConfig("myfile.json"); err != nil {
	log.Printf("error: %v", err) // the same error, a second time
}
```

**Good: wrap and return, let the caller decide:**

```go
func LoadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config %q: %w", path, err) // wrap with context, don't log
	}
	defer f.Close()
	// ... parse f ...
	return nil
}

// Caller decides to log once, at the boundary
if err := LoadConfig("myfile.json"); err != nil {
	log.Printf("failed to load config: %v", err) // one log message with full context
}
```

**Rule of thumb:**
- **Libraries:** Return errors. Let the caller decide how to handle them.
- **Main function / CLI:** Log errors (the top level of the call stack).
- **Middleware / boundaries:** Log interesting errors, then pass them on or return a safe response.

This way, each error is logged once, in context, with all the information needed to understand it.

**Related:** Never silently discard errors with `_ = someFunction()`. If you're ignoring an error intentionally, explain why in a comment:

```go
file, err := os.CreateTemp("", "scratch-*.txt")
if err != nil {
	log.Fatal(err)
}
defer os.Remove(file.Name())

// OK: intentional
_ = file.Close() // ignore close errors; the file is temporary

// Better: explicit comment
if err := file.Close(); err != nil {
	// Ignore: we're cleaning up a temporary file, not critical.
	fmt.Println("ignoring:", err) // this second Close fails: file already closed
}
```

### Closing and Flushing

When writing through a buffered writer, errors can happen at Flush or Close, not just Write. The `bufio.Writer` buffers data and only writes it to the underlying file when you call Flush. Errors can occur at each step:

1. `Write()`: buffer is full or the underlying write failed.
2. `Flush()`: force all buffered data to disk.
3. `Close()`: flush remaining data and close the file.

```go
func SaveFile(path string, data []byte) (err error) {
    f, err := os.Create(path)
    if err != nil {
        return err  // file creation failed; nothing to clean up
    }
    // Ensure Close is called even if we panic or error out
    defer func() {
        if cerr := f.Close(); cerr != nil {
            // If we already had an error, join both; otherwise just return the close error
            err = errors.Join(err, fmt.Errorf("close: %w", cerr))
        }
    }()

    w := bufio.NewWriter(f)
    if _, err := w.Write(data); err != nil {
        return fmt.Errorf("write: %w", err)  // return early; defer will still close the file
    }

    if err := w.Flush(); err != nil {
        return fmt.Errorf("flush: %w", err)  // flush may fail; defer will still close the file
    }

    return nil  // Close will be called by defer; if it fails, err will be set
}

fmt.Println(SaveFile(filepath.Join(os.TempDir(), "save-demo.txt"), []byte("hi"))) // <nil>
fmt.Println(SaveFile("/no/such/dir/file.txt", []byte("hi")))                      // the Create error
```

**Why this pattern matters:** Buffered writers hide when data actually hits the disk. If you ignore Flush or Close errors, data might not be written:

```go
dir, _ := os.MkdirTemp("", "flush-demo")
defer os.RemoveAll(dir)
data := []byte("hello\n")

// Bad: data might not be written
f1, _ := os.Create(filepath.Join(dir, "bad.txt"))
w1 := bufio.NewWriter(f1)
w1.Write(data)
f1.Close() // closed without Flush: the data is still in w1's buffer

// Good: ensure flush and close are called
f2, _ := os.Create(filepath.Join(dir, "good.txt"))
w2 := bufio.NewWriter(f2)
func() {
	defer func() {
		w2.Flush()
		f2.Close()
	}()
	w2.Write(data) // even if this panicked, flush and close would run
}()

bad, _ := os.ReadFile(filepath.Join(dir, "bad.txt"))
good, _ := os.ReadFile(filepath.Join(dir, "good.txt"))
fmt.Printf("bad: %q, good: %q\n", bad, good) // bad: "", good: "hello\n"
```

**Combining multiple errors:** If both Write and Close fail, join them so the caller can see both:

```go
func save(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data) // might fail
	closeErr := f.Close()        // might fail
	return errors.Join(writeErr, closeErr) // caller sees both; nil if both are nil
}

fmt.Println(save(filepath.Join(os.TempDir(), "join-demo.txt"), []byte("x"))) // <nil>
fmt.Println(save("/no/such/dir/file.txt", []byte("x")))                      // the Create error
```

### The Nil-Pointer-in-Interface Gotcha

A common bug when returning a typed nil pointer:

```go
type ParseError struct{ Line int }

func (e *ParseError) Error() string { return fmt.Sprintf("parse error on line %d", e.Line) }

func Parse(input string) (int, error) {
	var pe *ParseError // nil pointer
	n, err := strconv.Atoi(input)
	if err != nil {
		pe = &ParseError{Line: 1}
	}
	return n, pe // when pe is nil, the interface still holds (*ParseError, nil)
}

n, err := Parse("42")
if err != nil { // TRUE! the interface is non-nil
	fmt.Printf("n=%d, err != nil, err is %T(%v)\n", n, err, err)
}
```

Return `nil` explicitly:

```go
type ParseError struct{ Line int }

func (e *ParseError) Error() string { return fmt.Sprintf("parse error on line %d", e.Line) }

func Parse(input string) (int, error) {
	n, err := strconv.Atoi(input)
	if err != nil {
		return 0, &ParseError{Line: 1}
	}
	return n, nil // explicit nil
}

n, err := Parse("42")
fmt.Println(n, err == nil) // 42 true
```

## Idioms & Best Practices

- **Error as the last return:** `func F() (T, error)`. Check it right away.
- **Wrap with context:** Use `fmt.Errorf("operation X: %w", err)`. Put `%w` last. Don't repeat what the callee already said.
- **Unwrap carefully:** Never use `==` after wrapping; use `errors.Is`. For custom types, use `errors.As`.
- **Sentinel errors for matchable conditions:** `var ErrXxx = errors.New("...")` for expected, known failures. Name them `ErrXxx` (exported) or `errXxx` (unexported).
- **Custom types for extra data:** Use a struct with `Error()` and `Unwrap()` methods when you need to carry line numbers, field names, or other context.
- **errors.Join for multiple failures:** Validation, cleanup, and anything with independent errors.
- **Lowercase error messages:** "file not found", not "File Not Found" or "Error: file not found".
- **Handle once:** Log *or* return; if you log, return `nil`. If you return, the caller decides to log.
- **Never silently discard errors:** If you ignore an error with `_`, add a comment explaining why.
- **Defer cleanup on success too:** A file opened successfully must be closed, even if the next operation panics.
- **Catch panics only at boundaries:** HTTP handlers, job runners, RPC servers. Don't use `recover` in libraries.

## Common Pitfalls

1. **Comparing wrapped errors with `==`.** Use `errors.Is` instead.
2. **Hiding the error with `%v` when you should wrap with `%w`.** This breaks `errors.Is`.
3. **Panicking on user input.** Use errors and let the caller decide.
4. **Forgetting to check `Close` and `Flush` errors.** They can hide data loss.
5. **Logging and returning the same error.** The caller logs it again.
6. **Returning a nil-typed pointer through an error interface:** `var e *MyErr = nil; return e` (interface is non-nil). Return `nil` explicitly.
7. **Calling `errors.As` with a non-pointer target.** `errors.As(err, &myType)` requires the second argument to be a non-nil pointer.
8. **Wrapping at every layer with "failed to X: failed to Y: ..."** Be selective; avoid noise.

## Exercises

### 1. configparse: Sentinel and Custom Error Types

**Package:** `configparse`  
**File:** `modules/02-errors/exercises/configparse/`  
**Commands:**
```bash
go test ./modules/02-errors/exercises/configparse/
go test ./solutions/02-errors/exercises/configparse/
```

Parse a `key=value` configuration file. Return a `*ParseError` that records the line number and wraps one of several sentinel errors (`ErrMissingEquals`, `ErrEmptyKey`, `ErrInvalidKey`, `ErrDuplicateKey`). Implement `Error()` and `Unwrap()` so that:
- `errors.As(err, &pe)` extracts the line number.
- `errors.Is(err, ErrInvalidKey)` matches through wrapping.

Test that reader errors (disk failures) are wrapped with context but NOT reported as `*ParseError`.

### 2. lookup: Wrapping Through Layers

**Package:** `lookup`  
**File:** `modules/02-errors/exercises/lookup/`  
**Commands:**
```bash
go test ./modules/02-errors/exercises/lookup/
go test ./solutions/02-errors/exercises/lookup/
```

Implement a service layer on top of a storage layer. The storage returns a sentinel `ErrNotFound` wrapped with context ("user 7: not found"). The service layer either passes that up or handles it (e.g., in `Greet`, return "Hello, guest!" with nil error for not-found users). Show how sentinel errors and wrapping survive multiple layers, and how callers use `errors.Is` to match them.

### 3. validate: Multiple Errors with errors.Join

**Package:** `validate`  
**File:** `modules/02-errors/exercises/validate/`  
**Commands:**
```bash
go test ./modules/02-errors/exercises/validate/
go test ./solutions/02-errors/exercises/validate/
```

Validate a sign-up form (name, email, age) and report *all* problems at once using `errors.Join`. Each field has a `*FieldError` that wraps a sentinel (`ErrRequired`, `ErrTooLong`, `ErrInvalidFormat`, `ErrOutOfRange`). Return `nil` only if valid. Tests should check that `errors.Is(err, ErrRequired)` works and that `Fields(err)` returns the affected field names by unwrapping the join tree.

### 4. safediv: Panics vs Errors, and Recover

**Package:** `safediv`  
**File:** `modules/02-errors/exercises/safediv/`  
**Commands:**
```bash
go test ./modules/02-errors/exercises/safediv/
go test ./solutions/02-errors/exercises/safediv/
```

The `Divide(a, b)` function should return an error (not panic) for two cases where the `/` operator misbehaves: division by zero and integer overflow (`math.MinInt / -1`). It does not call any untrusted code, so no recover.

The `Safely(f)` function wraps a callback that might panic. It recovers, converts the panic to a `*PanicError` (which implements `Unwrap()` to expose the panic cause if it's an error), and returns it. This is appropriate at boundaries (HTTP handlers, job runners).

### 5. closeerr: Defer, Flush, and Close

**Package:** `closeerr`  
**File:** `modules/02-errors/exercises/closeerr/`  
**Commands:**
```bash
go test ./modules/02-errors/exercises/closeerr/
go test ./solutions/02-errors/exercises/closeerr/
```

Implement `WriteLines(w, lines)` that writes to a buffered writer, flushes, and closes—handling all errors including Close. Use a named return and defer to ensure Close runs. Join Close errors with earlier errors if both fail.

## Project: Bank Ledger

**Deadline:** ~ 45–60 minutes.

**Directory:** `modules/02-errors/project/`

Parse a transaction file (deposits, withdrawals, transfers) and report an account balance report. Transactions that fail (insufficient funds, invalid format) are reported in a summary, but processing continues. Exit code 0 if all succeed, 1 if any failed.

See `project/README.md` for the full spec.

When tests pass, ask your instructor (Claude) to review `modules/02-errors/`.

## Submitting for Review

When all tests pass, ask your instructor (Claude) to review `modules/02-errors/`.

## Further Reading

- [Go Blog: Errors are values](https://go.dev/blog/errors-are-values) — the philosophy.
- [Go Blog: Working with errors in Go 1.13](https://go.dev/blog/go1.13-errors) — wrapping, Is, As.
- [Code Review Comments: Error Strings](https://go.dev/wiki/CodeReviewComments#error-strings) — conventions.
- [Code Review Comments: Handle Errors](https://go.dev/wiki/CodeReviewComments#handle-errors) — error handling patterns.
- [Effective Go: Panic](https://go.dev/doc/effective_go#panic) — when to panic.
- [pkg.go.dev/errors](https://pkg.go.dev/errors) — API reference.
