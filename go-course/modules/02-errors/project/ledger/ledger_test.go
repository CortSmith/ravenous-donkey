package ledger

import (
	"errors"
	"maps"
	"math"
	"slices"
	"testing"
)

// newTestLedger returns a ledger with the given opening balances.
func newTestLedger(t *testing.T, balances map[string]int64) *Ledger {
	t.Helper()
	l := New()
	for _, name := range slices.Sorted(maps.Keys(balances)) {
		if err := l.Open(name, balances[name]); err != nil {
			t.Fatalf("Open(%q, %d) unexpected error: %v", name, balances[name], err)
		}
	}
	return l
}

// snapshot returns every balance in l.
func snapshot(t *testing.T, l *Ledger) map[string]int64 {
	t.Helper()
	out := make(map[string]int64)
	for _, name := range l.Accounts() {
		bal, err := l.Balance(name)
		if err != nil {
			t.Fatalf("Balance(%q) unexpected error: %v", name, err)
		}
		out[name] = bal
	}
	return out
}

// checkAccountError asserts err is an *AccountError with the given fields.
func checkAccountError(t *testing.T, err error, op, account string, cause error) {
	t.Helper()
	if err == nil {
		t.Fatalf("got nil error, want *AccountError{%s %s: %v}", op, account, cause)
	}
	var ae *AccountError
	if !errors.As(err, &ae) {
		t.Fatalf("error %v (%T) is not an *AccountError", err, err)
	}
	if ae.Op != op || ae.Account != account {
		t.Errorf("AccountError{Op: %q, Account: %q}, want {Op: %q, Account: %q}", ae.Op, ae.Account, op, account)
	}
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is(%v, %v) = false", err, cause)
	}
	if want := op + " " + account + ": " + cause.Error(); err.Error() != want {
		t.Errorf("err.Error() = %q, want %q", err.Error(), want)
	}
}

func TestOpenAndBalance(t *testing.T) {
	l := New()
	if err := l.Open("alice", 500); err != nil {
		t.Fatalf("Open(alice, 500) unexpected error: %v", err)
	}
	if err := l.Open("bob", 0); err != nil {
		t.Fatalf("Open(bob, 0) unexpected error: %v", err)
	}
	if got, err := l.Balance("alice"); err != nil || got != 500 {
		t.Errorf("Balance(alice) = %d, %v; want 500, nil", got, err)
	}
	if got := l.Accounts(); !slices.Equal(got, []string{"alice", "bob"}) {
		t.Errorf("Accounts() = %q, want [alice bob]", got)
	}

	checkAccountError(t, l.Open("alice", 1), "open", "alice", ErrAccountExists)
	checkAccountError(t, l.Open("carol", -1), "open", "carol", ErrInvalidAmount)

	_, err := l.Balance("zed")
	checkAccountError(t, err, "balance", "zed", ErrUnknownAccount)

	if got, _ := l.Balance("alice"); got != 500 {
		t.Errorf("failed Open changed alice's balance to %d", got)
	}
}

func TestAccountsEmpty(t *testing.T) {
	if got := New().Accounts(); len(got) != 0 {
		t.Errorf("New().Accounts() = %q, want empty", got)
	}
}

func TestDepositWithdraw(t *testing.T) {
	l := newTestLedger(t, map[string]int64{"alice": 1000})

	if err := l.Deposit("alice", 250); err != nil {
		t.Fatalf("Deposit unexpected error: %v", err)
	}
	if err := l.Withdraw("alice", 1250); err != nil { // exact balance is fine
		t.Fatalf("Withdraw of full balance unexpected error: %v", err)
	}
	if got, _ := l.Balance("alice"); got != 0 {
		t.Errorf("Balance(alice) = %d, want 0", got)
	}
}

func TestDepositWithdrawErrors(t *testing.T) {
	tests := []struct {
		name    string
		do      func(l *Ledger) error
		op      string
		account string
		cause   error
	}{
		{"deposit zero", func(l *Ledger) error { return l.Deposit("alice", 0) }, "deposit", "alice", ErrInvalidAmount},
		{"deposit negative", func(l *Ledger) error { return l.Deposit("alice", -5) }, "deposit", "alice", ErrInvalidAmount},
		{"deposit unknown", func(l *Ledger) error { return l.Deposit("zed", 5) }, "deposit", "zed", ErrUnknownAccount},
		{"deposit overflow", func(l *Ledger) error { return l.Deposit("rich", 1) }, "deposit", "rich", ErrOverflow},
		{"withdraw zero", func(l *Ledger) error { return l.Withdraw("alice", 0) }, "withdraw", "alice", ErrInvalidAmount},
		{"withdraw unknown", func(l *Ledger) error { return l.Withdraw("zed", 5) }, "withdraw", "zed", ErrUnknownAccount},
		{"withdraw too much", func(l *Ledger) error { return l.Withdraw("alice", 101) }, "withdraw", "alice", ErrInsufficientFunds},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newTestLedger(t, map[string]int64{"alice": 100, "rich": math.MaxInt64})
			before := snapshot(t, l)
			checkAccountError(t, tt.do(l), tt.op, tt.account, tt.cause)
			if after := snapshot(t, l); !maps.Equal(before, after) {
				t.Errorf("failed operation changed balances: before %v, after %v", before, after)
			}
		})
	}
}

func TestTransfer(t *testing.T) {
	l := newTestLedger(t, map[string]int64{"alice": 1000, "bob": 50})
	if err := l.Transfer("alice", "bob", 1000); err != nil {
		t.Fatalf("Transfer unexpected error: %v", err)
	}
	want := map[string]int64{"alice": 0, "bob": 1050}
	if got := snapshot(t, l); !maps.Equal(got, want) {
		t.Errorf("balances after transfer = %v, want %v", got, want)
	}
}

func TestTransferErrorsAreAtomic(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		amount   int64
		account  string // account named in the error
		cause    error
	}{
		{"zero amount", "alice", "bob", 0, "alice", ErrInvalidAmount},
		{"negative amount", "alice", "bob", -1, "alice", ErrInvalidAmount},
		{"same account", "alice", "alice", 1, "alice", ErrSameAccount},
		{"unknown source", "zed", "bob", 1, "zed", ErrUnknownAccount},
		{"unknown destination", "alice", "zed", 1, "zed", ErrUnknownAccount},
		{"insufficient funds", "alice", "bob", 101, "alice", ErrInsufficientFunds},
		{"destination overflow", "alice", "rich", 1, "rich", ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newTestLedger(t, map[string]int64{"alice": 100, "bob": 0, "rich": math.MaxInt64})
			before := snapshot(t, l)
			err := l.Transfer(tt.from, tt.to, tt.amount)
			checkAccountError(t, err, "transfer", tt.account, tt.cause)
			if after := snapshot(t, l); !maps.Equal(before, after) {
				t.Errorf("failed transfer is not atomic: before %v, after %v", before, after)
			}
		})
	}
}

func TestAccountErrorMethods(t *testing.T) {
	e := &AccountError{Op: "withdraw", Account: "bob", Err: ErrInsufficientFunds}
	if got, want := e.Error(), "withdraw bob: insufficient funds"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if e.Unwrap() != ErrInsufficientFunds {
		t.Errorf("Unwrap() = %v, want ErrInsufficientFunds", e.Unwrap())
	}
}
