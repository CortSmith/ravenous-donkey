// Package safediv contrasts the two ways a Go function can fail:
// returning an error (for expected failures) and panicking (for bugs), and
// shows the one place where recover is appropriate: a boundary that must
// keep running when a callback it does not control panics.
package safediv

import (
	"errors"
	"fmt"
	"math"
)

// Sentinel errors for the two cases where division misbehaves.
var (
	ErrDivideByZero = errors.New("division by zero")
	ErrOverflow     = errors.New("integer overflow")
)

// Divide returns a / b (truncated toward zero, like Go's / operator).
//
// It returns an error, never panics, for the two inputs where plain a / b
// misbehaves:
//   - b == 0: Go panics with "integer divide by zero" -> ErrDivideByZero.
//   - a == math.MinInt, b == -1: the true result (-MinInt) does not fit, and
//     Go silently returns MinInt (spec: "Integer overflow") -> ErrOverflow.
//
// Errors carry context: "divide 7 by 0: division by zero".
func Divide(a, b int) (int, error) {
	// TODO: Implement Divide.
	// Check for b == 0 and math.MinInt / -1 case.
	// Return an error with context.
	if b == 0 {
		return 0, fmt.Errorf("divide %d by %d: %w", a, b, ErrDivideByZero)
	}
	if a == math.MinInt && b == -1 {
		return 0, fmt.Errorf("divide %d by %d: %w", a, b, ErrOverflow)
	}
	return 0, errors.New("not implemented")
}

// PanicError is returned by Safely when the wrapped function panicked.
type PanicError struct {
	Value any    // the value passed to panic (as returned by recover)
	Stack []byte // goroutine stack captured at recovery time
}

// Error returns "panic: <value>".
func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v", e.Value) }

// Unwrap returns Value if it is an error (for example a runtime.Error such
// as an index out of range or nil dereference), and nil otherwise. This lets
// callers use errors.Is / errors.As on the panic cause.
func (e *PanicError) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}

// Safely calls f and returns its error. If f panics, Safely recovers and
// returns a *PanicError holding the panic value and stack trace instead of
// crashing the program. If f is nil, calling it panics, and that panic is
// converted too.
//
// Use this at boundaries (a plugin host, an HTTP handler, a job runner), not
// as a substitute for returning errors.
func Safely(f func() error) (err error) {
	// TODO: Implement Safely.
	// Use defer and recover to catch panics.
	// Return a *PanicError if a panic occurs.
	// Use runtime/debug.Stack() to capture the stack trace.
	return f()
}
