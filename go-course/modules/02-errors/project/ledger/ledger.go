// Package ledger implements a tiny bank ledger that processes a transaction
// file, keeps going when individual transactions fail, and reports every
// failure with structured, matchable errors.
package ledger

import (
	"errors"
)

// Sentinel causes. Ledger methods deliver them wrapped in *AccountError;
// Process delivers every per-line failure wrapped in *LineError.
var (
	ErrSyntax            = errors.New("syntax error")
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrUnknownAccount    = errors.New("unknown account")
	ErrAccountExists     = errors.New("account already exists")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrSameAccount       = errors.New("cannot transfer to the same account")
	ErrOverflow          = errors.New("balance overflow")
)

// AccountError reports a failed operation on a specific account.
type AccountError struct {
	Op      string // "open", "deposit", "withdraw", "transfer" or "balance"
	Account string // the account the problem is about
	Err     error  // one of the sentinels above
}

// Error returns "op account: cause", e.g. "withdraw bob: insufficient funds".
func (e *AccountError) Error() string { return e.Op + " " + e.Account + ": " + e.Err.Error() }

// Unwrap returns the cause.
func (e *AccountError) Unwrap() error { return e.Err }

// Ledger holds account balances in cents. The zero value is not usable;
// call New.
type Ledger struct {
	balances map[string]int64
}

// New returns an empty ledger.
func New() *Ledger {
	// TODO: Implement New.
	return nil
}

// Open creates an account with the given initial balance (>= 0).
func (l *Ledger) Open(account string, initial int64) error {
	// TODO: Implement Open.
	return errors.New("not implemented")
}

// Balance returns the balance of account in cents.
func (l *Ledger) Balance(account string) (int64, error) {
	// TODO: Implement Balance.
	return 0, errors.New("not implemented")
}

// Accounts returns all account names in sorted order.
func (l *Ledger) Accounts() []string {
	// TODO: Implement Accounts.
	return nil
}

// Deposit adds amount (> 0) to account.
func (l *Ledger) Deposit(account string, amount int64) error {
	// TODO: Implement Deposit.
	return errors.New("not implemented")
}

// Withdraw removes amount (> 0) from account. Balances never go negative.
func (l *Ledger) Withdraw(account string, amount int64) error {
	// TODO: Implement Withdraw.
	return errors.New("not implemented")
}

// Transfer moves amount (> 0) from one account to another. It is atomic:
// every check runs before any balance changes, so a failed transfer leaves
// both accounts untouched. The error's Account is the account at fault
// (the destination for an unknown or overflowing destination, the source
// otherwise).
func (l *Ledger) Transfer(from, to string, amount int64) error {
	// TODO: Implement Transfer.
	return errors.New("not implemented")
}
