# Module 02-Errors: Review Rubric

## Exercise 1: configparse

### Readable Solution

The `configparse` package demonstrates sentinel errors, custom error types, and proper error wrapping.

**Key points:**

- `ParseError` struct holds `Line`, `Key` (optional), and `Err` (an underlying sentinel).
- `Error()` formats as `"line N: ..."` or `"line N: key \"k\": ..."`.
- `Unwrap()` returns the underlying sentinel, so `errors.Is(err, ErrMissingEquals)` works through wrapping.
- The `validKey` helper ensures keys match ASCII [a-zA-Z0-9_.-].
- Reader errors are wrapped with context (e.g., `"read config: %w"`) but NOT reported as `*ParseError`, preserving the distinction between syntax errors and I/O errors.
- `errors.As` can extract the `ParseError` even after a caller wraps it further, demonstrating the robustness of the design.

**Trade-offs:**

- The readable version allocates a new `ParseError` each time a validation fails, which is fine for a config parser (runs once at startup).
- The `fast.go` version is available for comparison: it preallocates sentinel `ParseError` instances, reducing allocations. Benchmark (measured on go1.24.2 linux/amd64): readable ~= 57.96 µs with 426 allocs for 200 lines (~0.29 µs/line, 2.13 allocs/line), fast ~= 37.89 µs with 226 allocs (~0.19 µs/line, 1.13 allocs/line).

### Optimized Solution

See `exercises/configparse/fast.go`. It:

- Pre-allocates sentinel `ParseError` instances for static error messages (e.g., `errMissingEquals`).
- Reuses the same `*ParseError` pointer when a validation fails (works because the cause is not modified by callers).
- **Caveat:** If a caller does `errors.As(err, &pe); pe.Key = "x"`, this corrupts the shared error. The README mentions this trade-off.
- Returns `nil` early if the input is valid, avoiding a call to `errors.Join`.

### Grading Checklist

- [ ] Sentinel errors are declared (ErrMissingEquals, ErrEmptyKey, ErrInvalidKey, ErrDuplicateKey).
- [ ] ParseError is a struct with Line, Key, Err fields.
- [ ] ParseError.Error() returns the correct format.
- [ ] ParseError.Unwrap() is implemented.
- [ ] validKey rejects non-ASCII and spaces.
- [ ] Parse reads lines correctly and skips comments and blanks.
- [ ] Duplicate keys are detected (with the line they were first defined on in the error message).
- [ ] Reader errors are wrapped with context and not reported as *ParseError.
- [ ] First error wins (parsing stops at the first problem).

---

## Exercise 2: lookup

### Readable Solution

The `lookup` package demonstrates:

- Sentinel errors flowing through multiple layers.
- Wrapping with context at each layer, e.g., `"user 7: not found"`.
- Using `errors.Is` to match sentinels even after wrapping.
- A Service layer that handles "not found" as an expected condition (e.g., Greet returns "Hello, guest!" with nil error).

**Key points:**

- `MapStore.Get` returns an error wrapping `ErrNotFound` with context about which user was not found.
- `Service.DisplayName` calls `Get` and wraps its error again, adding "user store: " context.
- `Service.Greet` treats `ErrNotFound` as success (returns "Hello, guest!") but passes through other errors.
- The key idiom: `errors.Is(err, ErrNotFound)` works even when the error is wrapped multiple times.

### Optimized Solution

No significant optimization for this exercise; the readable solution is the idiomatic approach.

### Grading Checklist

- [ ] Sentinel errors are defined (ErrNotFound, ErrInvalidID).
- [ ] User struct is defined with ID and Name.
- [ ] Getter interface is small and defined by the consumer (Service package).
- [ ] MapStore.Get returns errors wrapping ErrNotFound with the user ID.
- [ ] Service.DisplayName validates the ID, calls Get, and wraps the error.
- [ ] Service.Greet uses errors.Is to match ErrNotFound and treats it as success.
- [ ] Errors wrap through multiple layers but remain matchable.

---

## Exercise 3: validate

### Readable Solution

The `validate` package demonstrates:

- Multiple independent validation errors collected and joined with `errors.Join`.
- Custom error type (`FieldError`) with a field name and a sentinel cause.
- Each field validated once; only the first failure per field is reported.
- The `Fields` helper unwraps an `errors.Join` tree and a `fmt.Errorf` chain to extract field names.

**Key points:**

- `Validate` calls `checkName`, `checkEmail`, and `checkAge` independently, collecting their errors.
- `errors.Join` is called with all errors (including nils), which returns nil if all are nil.
- `errors.Is(err, ErrRequired)` works through the join and returns true if any joined branch contains `ErrRequired`.
- The `Fields` function walks through both `Unwrap() error` (for `fmt.Errorf` chains) and `Unwrap() []error` (for `errors.Join`).

### Optimized Solution

See `exercises/validate/fast.go`:

- Pre-allocates static `FieldError` instances for static field/sentinel combinations.
- Checks all fields first, then only calls `errors.Join` if there are failures (avoid the join overhead on the happy path).
- Uses a byte-length shortcut for the name length check (a string of at most MaxNameLen bytes cannot exceed MaxNameLen runes).

**Benchmark (measured on go1.24.2 linux/amd64):** Happy path (valid input): readable ~= 61.45 ns with 0 allocs, fast ~= 44.90 ns with 0 allocs. Invalid input: readable ~= 174.9 ns with 4 allocs, fast ~= 110.1 ns with 2 allocs.

### Grading Checklist

- [ ] Sentinel errors are declared (ErrRequired, ErrTooLong, ErrInvalidFormat, ErrOutOfRange).
- [ ] Constants MaxNameLen, MinAge, MaxAge are defined.
- [ ] FieldError struct is defined with Field and Err.
- [ ] FieldError.Error() and Unwrap() are implemented.
- [ ] Validate returns nil for valid input, errors.Join of failures otherwise.
- [ ] Name is required, trimmed, and checked for max rune length.
- [ ] Email is required, trimmed, and validated (no whitespace, one '@', non-empty local, domain with dot not starting/ending with dot).
- [ ] Age is in [MinAge, MaxAge].
- [ ] Age error includes the actual value in the message.
- [ ] Fields helper correctly extracts field names from join trees and fmt.Errorf chains.
- [ ] nil *FieldError in the error interface is not returned (values are returned, not pointers).

---

## Exercise 4: safediv

### Readable Solution

The `safediv` package demonstrates two important patterns:

1. **Errors for expected failures** (division by zero, overflow): `Divide` returns errors for cases where `/` misbehaves.
2. **Recover for boundary protection** (panics in user callbacks): `Safely` recovers from panics in a user-provided function.

**Key points:**

- `Divide` checks for division by zero (b == 0) and overflow (a == math.MinInt && b == -1) *before* computing, returning errors instead of panicking.
- `PanicError` holds the panic value (any) and a stack trace.
- `PanicError.Unwrap()` returns the panic value if it's an error (e.g., a runtime.Error), enabling `errors.Is` matching.
- `Safely` uses a deferred function with `recover()`, capturing the panic value and stack trace, converting it to a `*PanicError`.

### Optimized Solution

No significant optimization; the readable version is idiomatic.

### Grading Checklist

- [ ] Sentinel errors are declared (ErrDivideByZero, ErrOverflow).
- [ ] Divide checks for b == 0 and math.MinInt / -1 cases.
- [ ] Divide returns an error with context, never panics.
- [ ] PanicError struct has Value (any) and Stack ([]byte) fields.
- [ ] PanicError.Error() returns "panic: <value>".
- [ ] PanicError.Unwrap() returns the value as an error if it is one, nil otherwise.
- [ ] Safely catches panics and returns *PanicError.
- [ ] Safely captures the stack trace with runtime/debug.Stack().
- [ ] A panic in Safely does not escape.
- [ ] Ordinary errors returned by the function are passed through unchanged (not wrapped in *PanicError).

---

## Exercise 5: closeerr

### Readable Solution

The `closeerr` package demonstrates **proper defer usage for cleanup on all paths**.

**Key points:**

- `WriteLines` uses a named return variable `err` and a deferred function that captures `err` by pointer.
- The deferred function checks `w.Close()` and, if it fails, joins the close error with any prior write/flush error.
- `bw.Flush()` is called explicitly; a failed write or flush returns early with the error wrapped (e.g., `"write: ..."`).
- If both a write/flush error and a close error occur, `errors.Join` combines them so both are visible to `errors.Is`.
- The file is closed on every path: successful writes, write failures, and even if WriteLines panics (because `defer` runs).

### Optimized Solution

No significant optimization; the readable version is the idiomatic pattern.

### Grading Checklist

- [ ] WriteLines uses a named return variable.
- [ ] WriteLines defers w.Close() in a function that can modify the named return.
- [ ] The deferred function joins the close error with the write/flush error (if any).
- [ ] Write and Flush errors are wrapped with context ("write: ...", "flush: ...").
- [ ] Close error is wrapped with context ("close: ...").
- [ ] Close is called exactly once, even on error.
- [ ] Multiple errors (write + close) are joined so both are matchable.
- [ ] SaveFile calls os.Create and returns its error for file-not-found etc.
- [ ] SaveFile delegates the write/close logic to WriteLines.

---

## Project: Bank Ledger

### Readable Solution

The ledger demonstrates:

- Custom error types (`AccountError`, `LineError`) with structured data (operation, account, line number).
- Sentinel errors for matchable causes (ErrInsufficientFunds, ErrUnknownAccount, etc.).
- Wrapping errors with context at the appropriate layer.
- Continuing on errors (Process keeps going after a failed transaction, collecting all failures).
- Atomic transfers (all checks run before any balance changes).

**Key design:**

- `Ledger.Transfer` is atomic: it checks both accounts, amounts, and overflow *before* any balance changes. If any check fails, both accounts are left unchanged.
- `Process` reads lines, parses commands, and applies them. Each failure is recorded as a `*LineError` (line number + cause), and processing continues.
- `Result.Err()` returns `errors.Join` of all failures, or nil if there are none.
- `Run` is a testable CLI that can accept stdin or a file, prints failures (unless -q), and returns the correct exit code.

### Optimized Solution

The readable solution is efficient; no major optimization needed. Possible improvements:

- Batch processing (apply multiple transactions in one critical section under a mutex, if concurrency were added).
- Caching balances to avoid map lookups.

### Grading Checklist: Ledger Type

- [ ] New() returns an initialized ledger.
- [ ] Open(account, initial) creates an account with the given balance, returns error if the account exists or balance is negative.
- [ ] Balance(account) returns the balance, returns error if the account doesn't exist.
- [ ] Accounts() returns all account names in sorted order.
- [ ] Deposit(account, amount) adds to an account, validates amount > 0, account exists, no overflow.
- [ ] Withdraw(account, amount) subtracts from an account, validates amount > 0, account exists, sufficient funds.
- [ ] Transfer(from, to, amount) is atomic (all checks before any changes), validates both accounts, amount > 0, from != to, sufficient funds, no overflow. Reports the account at fault.

### Grading Checklist: Error Types

- [ ] AccountError struct has Op, Account, Err.
- [ ] AccountError.Error() and Unwrap() are implemented correctly.
- [ ] Sentinel errors are declared and used consistently.

### Grading Checklist: Process and Parsing

- [ ] LineError struct has Line (1-based), Err.
- [ ] LineError.Error() and Unwrap() are implemented.
- [ ] Result struct has Applied, Failures.
- [ ] Result.Err() returns errors.Join of failures or nil.
- [ ] Process skips comments and blank lines.
- [ ] Process parses each command correctly and applies it to the ledger.
- [ ] Process continues on failure, recording the error.
- [ ] Process detects syntax errors (unknown command, wrong number of arguments).
- [ ] ParseAmount parses decimal amounts ("12.34", "0.05") as cents.
- [ ] ParseAmount rejects invalid formats (signs, exponents, trailing dot, overflow).
- [ ] FormatAmount formats cents as a decimal with exactly 2 digits.

### Grading Checklist: CLI (Run)

- [ ] Run parses the -q flag.
- [ ] Run accepts zero or one positional argument (the file).
- [ ] Run reads from stdin if FILE is absent or "-".
- [ ] Run opens FILE and returns ExitIO on open errors.
- [ ] Run prints all failures to stderr unless -q.
- [ ] Run prints final balances to stdout.
- [ ] Run prints a summary to stderr.
- [ ] Run returns ExitOK if no failures, ExitTxFailed if any failed.
- [ ] Run returns ExitIO on read errors.
- [ ] Run returns ExitUsage on bad command-line usage or flag parse errors (unless -h, which returns ExitOK).

---

## Common Pitfalls and Grading Notes

### Across All Exercises

1. **Comparing wrapped errors with ==**: Always use `errors.Is` after wrapping. Direct `==` comparison fails.
2. **Ignoring the Unwrap() method**: Without `Unwrap()`, `errors.Is` and `errors.As` won't work through custom error types.
3. **Nil pointer in interface**: A function that returns `var e *MyErr = nil; return e` will return a non-nil error interface. Return `nil` explicitly.
4. **Hiding errors with %v instead of %w**: Using `fmt.Errorf("%v", err)` loses the ability to match the wrapped error with `errors.Is`. Always use `%w`.
5. **Capitalizing error messages**: Error messages should start lowercase and have no trailing period.
6. **Redundant "Error: " prefix**: Don't prefix error strings with "Error: " or "Failed to ...".

### configparse

- Ensure `validKey` rejects non-ASCII characters, including UTF-8.
- The test for line-too-long (bufio.ErrTooLong) checks that sc.Err() is consulted; a common bug is to not check it.
- Reader errors must be wrapped with context but NOT reported as *ParseError (test checks this distinction).

### lookup

- The sentinel errors must survive wrapping through multiple layers.
- errors.Is works because Unwrap() is implemented.

### validate

- A valid Signup with surrounding whitespace must succeed (trim before checking).
- Email validation is strict: no whitespace, exactly one '@', non-empty local and domain, domain must have a dot not at the edges.
- A nil *FieldError in the error interface is a bug (always return concrete errors from helpers, not typed nil pointers).
- The Fields helper must walk through both Unwrap() error and Unwrap() []error chains.

### safediv

- Divide must never panic; it checks for the two edge cases.
- Safely must actually recover (not just catch); test verifies no panic escapes.
- PanicError.Unwrap() returns the value as an error if it is one (enabling errors.As on runtime.Error).

### closeerr

- Close must be called exactly once, even on error.
- A close error that occurs alongside a write error must be joinable; both must be matchable with errors.Is.
- SaveFile must return the PathError from os.Create unchanged (it already has the path and operation).

### Bank Ledger Project

- Balances are stored as int64 cents, not floats.
- Transfer is atomic: all checks before any state changes.
- Process must continue on failure (every transaction is attempted).
- LineError must record the 1-based line number.
- Exit codes are ExitOK (0), ExitTxFailed (1), ExitUsage (2), ExitIO (3).
- The -q flag suppresses individual failure messages but not the summary.

---

## Suggested Improvements for Student Code

If a student's solution works but could be better:

1. **Over-wrapping**: If every function wraps the error, the chain becomes too deep. Be selective: wrap where you add meaningful context, not at every layer.
2. **Missing Unwrap()**: If a custom error type implements Error() but not Unwrap(), errors.Is and errors.As won't work through it.
3. **Panic in libraries**: If a student uses panic for expected failures (e.g., invalid input), suggest returning an error instead. Panic is for programmer bugs.
4. **Ignoring Close errors**: If a student opens a file or creates a resource but doesn't check Close, note that Close can hide data loss (especially on writes).
5. **String-matching errors**: If a student does `strings.Contains(err.Error(), "not found")` instead of using `errors.Is`, point out the fragility.

---

## Testing Notes

- All tests must pass: `go test ./solutions/02-errors/...`
- All tests must pass with `-race`: `go test -race ./solutions/02-errors/...` (though there is no concurrency in these exercises).
- `gofmt` must pass: `gofmt -l ./solutions/02-errors/` should print nothing.
- `go vet` must pass: `go vet ./solutions/02-errors/...` should print nothing.
