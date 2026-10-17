package ledger

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
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
	var res Result
	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := l.apply(strings.Fields(line)); err != nil {
			res.Failures = append(res.Failures, &LineError{Line: lineNo, Err: err})
			continue
		}
		res.Applied++
	}
	if err := sc.Err(); err != nil {
		return res, fmt.Errorf("read transactions: %w", err)
	}
	return res, nil
}

// apply executes one tokenized transaction.
func (l *Ledger) apply(fields []string) error {
	cmd, args := fields[0], fields[1:]
	use, known := usage[cmd]
	if !known {
		return fmt.Errorf("%w: unknown command %q", ErrSyntax, cmd)
	}
	wrongArgs := func() error { return fmt.Errorf("%w: usage: %s", ErrSyntax, use) }

	switch cmd {
	case "open":
		if len(args) < 1 || len(args) > 2 {
			return wrongArgs()
		}
		var initial int64
		if len(args) == 2 {
			var err error
			if initial, err = ParseAmount(args[1]); err != nil {
				return err
			}
		}
		return l.Open(args[0], initial)

	case "deposit", "withdraw":
		if len(args) != 2 {
			return wrongArgs()
		}
		amount, err := ParseAmount(args[1])
		if err != nil {
			return err
		}
		if cmd == "deposit" {
			return l.Deposit(args[0], amount)
		}
		return l.Withdraw(args[0], amount)

	default: // "transfer"
		if len(args) != 3 {
			return wrongArgs()
		}
		amount, err := ParseAmount(args[2])
		if err != nil {
			return err
		}
		return l.Transfer(args[0], args[1], amount)
	}
}
