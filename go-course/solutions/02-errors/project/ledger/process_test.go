package ledger

import (
	"bufio"
	"errors"
	"io"
	"maps"
	"strings"
	"testing"
	"testing/iotest"
)

func TestProcessAllValid(t *testing.T) {
	input := `
# comment
open alice 100
open bob
	deposit   alice   0.50
transfer alice bob 25.25
withdraw bob 5
`
	l := New()
	res, err := l.Process(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Process unexpected error: %v", err)
	}
	if res.Applied != 5 {
		t.Errorf("Applied = %d, want 5", res.Applied)
	}
	if len(res.Failures) != 0 {
		t.Errorf("Failures = %v, want none", res.Failures)
	}
	if err := res.Err(); err != nil {
		t.Errorf("Result.Err() = %v (%T), want nil", err, err)
	}
	want := map[string]int64{"alice": 7525, "bob": 2025}
	if got := snapshot(t, l); !maps.Equal(got, want) {
		t.Errorf("balances = %v, want %v", got, want)
	}
}

func TestProcessContinuesAfterFailures(t *testing.T) {
	input := strings.Join([]string{
		"open alice 10",          // 1 ok
		"withdraw alice 20",      // 2 insufficient
		"deposit bob 5",          // 3 unknown account
		"refund alice 1",         // 4 unknown command
		"open",                   // 5 wrong arg count
		"deposit alice 1.234",    // 6 bad amount
		"transfer alice alice 1", // 7 same account
		"open alice",             // 8 exists
		"transfer alice x y z",   // 9 wrong arg count
		"open bob 1 2",           // 10 wrong arg count
		"deposit alice 0",        // 11 zero amount rejected by Deposit
		"withdraw alice 2",       // 12 ok
	}, "\n")

	type want struct {
		line  int
		cause error
		msg   string // exact message; "" to skip
	}
	wants := []want{
		{2, ErrInsufficientFunds, "line 2: withdraw alice: insufficient funds"},
		{3, ErrUnknownAccount, "line 3: deposit bob: unknown account"},
		{4, ErrSyntax, `line 4: syntax error: unknown command "refund"`},
		{5, ErrSyntax, "line 5: syntax error: usage: open ACCOUNT [AMOUNT]"},
		{6, ErrInvalidAmount, `line 6: invalid amount "1.234"`},
		{7, ErrSameAccount, "line 7: transfer alice: cannot transfer to the same account"},
		{8, ErrAccountExists, "line 8: open alice: account already exists"},
		{9, ErrSyntax, "line 9: syntax error: usage: transfer FROM TO AMOUNT"},
		{10, ErrSyntax, ""},
		{11, ErrInvalidAmount, "line 11: deposit alice: invalid amount"},
	}

	l := New()
	res, err := l.Process(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Process unexpected error: %v", err)
	}
	if res.Applied != 2 {
		t.Errorf("Applied = %d, want 2", res.Applied)
	}
	if len(res.Failures) != len(wants) {
		t.Fatalf("got %d failures, want %d:\n%v", len(res.Failures), len(wants), res.Err())
	}
	for i, w := range wants {
		f := res.Failures[i]
		if f.Line != w.line {
			t.Errorf("Failures[%d].Line = %d, want %d", i, f.Line, w.line)
		}
		if !errors.Is(f, w.cause) {
			t.Errorf("Failures[%d] = %v; errors.Is(_, %v) = false", i, f, w.cause)
		}
		if w.msg != "" && f.Error() != w.msg {
			t.Errorf("Failures[%d].Error() = %q, want %q", i, f.Error(), w.msg)
		}
	}
	if got, _ := l.Balance("alice"); got != 800 {
		t.Errorf("Balance(alice) = %d, want 800", got)
	}

	// Operation failures keep their *AccountError inside the *LineError.
	var ae *AccountError
	if !errors.As(res.Failures[0], &ae) || ae.Account != "alice" || ae.Op != "withdraw" {
		t.Errorf("errors.As(Failures[0], *AccountError) = %+v; want withdraw alice", ae)
	}
	// ...but syntax errors are not account errors.
	if errors.As(res.Failures[2], &ae) {
		t.Errorf("syntax failure %v unexpectedly contains an *AccountError", res.Failures[2])
	}

	// Result.Err joins everything and stays matchable.
	joined := res.Err()
	for _, sentinel := range []error{ErrInsufficientFunds, ErrSyntax, ErrInvalidAmount, ErrAccountExists} {
		if !errors.Is(joined, sentinel) {
			t.Errorf("errors.Is(Result.Err(), %v) = false", sentinel)
		}
	}
	var le *LineError
	if !errors.As(joined, &le) || le.Line != 2 {
		t.Errorf("errors.As(Result.Err(), *LineError) = %v; want the first failure (line 2)", le)
	}
	if n := strings.Count(joined.Error(), "\n") + 1; n != len(wants) {
		t.Errorf("Result.Err() has %d lines, want %d", n, len(wants))
	}
}

func TestProcessEmpty(t *testing.T) {
	res, err := New().Process(strings.NewReader(""))
	if err != nil || res.Applied != 0 || len(res.Failures) != 0 || res.Err() != nil {
		t.Errorf("Process(\"\") = %+v, %v; want zero Result, nil", res, err)
	}
}

func TestProcessReadError(t *testing.T) {
	errDisk := errors.New("disk on fire")
	r := io.MultiReader(strings.NewReader("open a 1\nopen b 2\n"), iotest.ErrReader(errDisk))

	l := New()
	res, err := l.Process(r)
	if !errors.Is(err, errDisk) {
		t.Fatalf("Process(failing reader) error = %v, want it to wrap %v", err, errDisk)
	}
	if !strings.HasPrefix(err.Error(), "read transactions: ") {
		t.Errorf("error = %q, want prefix %q", err.Error(), "read transactions: ")
	}
	var le *LineError
	if errors.As(err, &le) {
		t.Errorf("read failure reported as *LineError: %v", le)
	}
	if res.Applied != 2 {
		t.Errorf("Applied before read failure = %d, want 2 (keep partial results)", res.Applied)
	}
}

func TestProcessLineTooLong(t *testing.T) {
	input := "open a 1\nopen " + strings.Repeat("x", bufio.MaxScanTokenSize) + "\n"
	_, err := New().Process(strings.NewReader(input))
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("Process(huge line) error = %v, want bufio.ErrTooLong (check sc.Err())", err)
	}
}

func TestLineErrorMethods(t *testing.T) {
	cause := &AccountError{Op: "deposit", Account: "x", Err: ErrUnknownAccount}
	e := &LineError{Line: 12, Err: cause}
	if got, want := e.Error(), "line 12: deposit x: unknown account"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if e.Unwrap() != cause {
		t.Errorf("Unwrap() = %v, want %v", e.Unwrap(), cause)
	}
}
