# Project: Bank Ledger

**Deadline:** ~ 45–60 minutes.

## Overview

Implement a bank ledger that processes a file of transactions (open account, deposit, withdraw, transfer) and reports successful and failed transactions. Unlike the exercises, the ledger must:

- Define structured, matchable error types for specific failures (insufficient funds, syntax errors, etc.)
- Keep going when a transaction fails, reporting *all* failures at the end (not stopping on the first error).
- Use typed errors (`*AccountError` and `*LineError`) so callers can match specific problems with `errors.Is` and `errors.As`.
- Report an exit code: 0 if all transactions succeeded, 1 if any failed.

## Spec

### Ledger API (`ledger.go`)

Define a `Ledger` type that manages accounts and their balances. Balances are stored in **cents** (as `int64`), so there are no floating-point issues.

**Exported functions and types:**

- `New() *Ledger`: Return an empty ledger.
- `Open(account string, initial int64) error`: Create a new account with the given initial balance (>= 0). If the account exists, return error. If the initial amount is negative, return error.
- `Balance(account string) (int64, error)`: Return the balance in cents. Return an error if the account doesn't exist.
- `Accounts() []string`: Return all account names in sorted order.
- `Deposit(account string, amount int64) error`: Add cents to an account. Return error if the account doesn't exist, the amount is invalid (≤ 0), or overflow would occur.
- `Withdraw(account string, amount int64) error`: Subtract cents. Return error if the account doesn't exist, the amount is invalid (≤ 0), or there are insufficient funds.
- `Transfer(from, to string, amount int64) error`: Move cents from one account to another. This is atomic: all checks must pass before any balance changes. Return error for unknown accounts, invalid amounts, insufficient funds, attempting to transfer to the same account, or overflow on the destination. Report the account at fault in the error message.

**Error types:**

- `AccountError`: A struct with fields `Op` (string: "open", "deposit", "withdraw", "balance", "transfer"), `Account` (string), and `Err` (error). It implements `Error()` and `Unwrap()`. The error message format is `"op account: cause"`, e.g., `"deposit alice: insufficient funds"`.
- Sentinel errors: `ErrAccountExists`, `ErrUnknownAccount`, `ErrInsufficientFunds`, `ErrInvalidAmount`, `ErrSameAccount`, `ErrOverflow`, `ErrSyntax`. Each is created with `errors.New` and wrapped in an `*AccountError` by ledger operations.

Amounts are validated: they must be > 0 for deposits, withdrawals, and transfers.

### File Parsing (`process.go`)

Implement `Process(r io.Reader) (Result, error)` on `*Ledger`.

**Input format:** One transaction per line:
```
open alice 100      # Open alice with 100 cents. Amount is optional; defaults to 0.
open bob            # Open bob with 0.
deposit alice 50    # Add 50 to alice.
withdraw bob 25     # Subtract 25 from bob.
transfer alice bob 30  # Move 30 from alice to bob.
# Lines starting with '#' are comments.
               # Blank lines are ignored.
```

Fields are separated by whitespace.

**Error handling:** If a transaction line is malformed (bad syntax, bad amount, or an operation fails), record the error in `Result.Failures` as a `*LineError` and keep processing. Do not stop.

**Result type:**
```go
// Runs YOUR ledger package. The types are already in process.go:
//
//	type Result struct {
//		Applied  int          // successful transactions
//		Failures []*LineError // one per failed transaction
//	}
//
// LineError has Line (1-based) and Err (the cause: a syntax error or an
// *AccountError), with Error() and Unwrap(). Result.Err() returns
// errors.Join of all failures, or nil if there are none.
import "gocourse/modules/02-errors/project/ledger"

input := "open alice 100\nwithdraw alice 500\nfly bob\ndeposit alice 5\n"
res, err := ledger.New().Process(strings.NewReader(input))
fmt.Println("applied:", res.Applied, "failures:", len(res.Failures), "read error:", err)
fmt.Println(res.Err())
// When you're done: applied: 2 failures: 2, then one line per failure.
```

If reading the input fails (e.g., I/O error), return the read error wrapped in context (e.g., `"read transactions: ..."`) and the Result with what was processed so far.

### CLI (`cmd/ledger/main.go` and `ledger.go` Run function)

Implement `Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int` on package ledger.

The CLI:
- Takes a file path as an argument. If no argument, read from stdin.
- Calls `Process` to apply transactions.
- Prints a summary: number of successful transactions and a report of all failures (line number, error).
- Returns exit code 0 if all succeeded, 1 if any failed.
- Returns exit code 2 if the input file cannot be read or parsed in a fatal way (e.g., cannot open the file).

## Acceptance Criteria

- [ ] All ledger operations (`Open`, `Deposit`, `Withdraw`, `Transfer`, `Balance`) work correctly and return errors for invalid inputs.
- [ ] Error types (`AccountError`, `LineError`) are properly defined with `Error()` and `Unwrap()` methods.
- [ ] Sentinel errors are matchable with `errors.Is`.
- [ ] `Process` keeps going on failed transactions, collecting all failures in order.
- [ ] `Run` prints all failures in a readable format.
- [ ] Exit code is 0 on success, 1 on transaction failures, 2 on fatal input errors.
- [ ] Tests pass: `go test ./modules/02-errors/project/...`
- [ ] No data races: `go test -race ./modules/02-errors/project/...`

## Testing

Run tests from the course root:
```bash
go test ./modules/02-errors/project/...
go test -race ./modules/02-errors/project/...
```

## Stretch Goals

- Persist the ledger to a file and reload it (JSON or newline-delimited format).
- Support comments in transaction files (already do this).
- Add a `QueryBalance` command that outputs the final balance of an account.
- Add undo/rollback for the last transaction.
