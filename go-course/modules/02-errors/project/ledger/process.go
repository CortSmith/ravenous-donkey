package ledger

import (
	"errors"
	"fmt"
	"io"
)

// LineError reports a failed transaction at a given line of the input.
type LineError struct {
	Line int   // 1-based
	Err  error // a syntax/amount error or an *AccountError
}

// Error returns "line N: cause".
func (e *LineError) Error() string { return fmt.Sprintf("line %d: %v", e.Line, e.Err) }

// Unwrap returns the cause.
func (e *LineError) Unwrap() error { return e.Err }

// Result summarises a Process run.
type Result struct {
	Applied  int          // transactions that succeeded
	Failures []*LineError // one per failed transaction, in input order
}

// Err returns all failures joined with errors.Join, or nil if there were
// none. (It must return a literal nil, not a nil slice wrapped in an error.)
func (r Result) Err() error {
	if len(r.Failures) == 0 {
		return nil
	}
	errs := make([]error, len(r.Failures))
	for i, f := range r.Failures {
		errs[i] = f
	}
	return errors.Join(errs...)
}

// usage lists the accepted forms of each command for syntax-error messages.
var usage = map[string]string{
	"open":     "open ACCOUNT [AMOUNT]",
	"deposit":  "deposit ACCOUNT AMOUNT",
	"withdraw": "withdraw ACCOUNT AMOUNT",
	"transfer": "transfer FROM TO AMOUNT",
}

// Process applies the transactions read from r, one per line:
//
//	open ACCOUNT [AMOUNT]      # AMOUNT defaults to 0
//	deposit ACCOUNT AMOUNT
//	withdraw ACCOUNT AMOUNT
//	transfer FROM TO AMOUNT
//
// Fields are separated by any whitespace. Blank lines and lines starting
// with '#' are ignored. A line that fails (bad syntax, bad amount, or a
// rejected operation) is recorded in Result.Failures as a *LineError and
// processing CONTINUES with the next line.
//
// The returned error is non-nil only if reading r fails; it then wraps the
// read error ("read transactions: ...") and Result holds what was processed
// before the failure.
func (l *Ledger) Process(r io.Reader) (Result, error) {
	// TODO: Implement Process.
	// - Use bufio.NewScanner to read lines.
	// - Skip blank lines and comments.
	// - Parse and apply each transaction with apply().
	// - On error, append *LineError to res.Failures and continue.
	// - Check sc.Err() after the loop.
	return Result{}, errors.New("not implemented")
}

// apply executes one tokenized transaction.
func (l *Ledger) apply(fields []string) error {
	// TODO: Implement apply.
	// Parse the command (fields[0]) and dispatch to Open, Deposit, Withdraw, Transfer.
	// Return ErrSyntax for unknown commands or wrong argument counts.
	return errors.New("not implemented")
}
