// Package validate checks a sign-up form and reports EVERY problem at once,
// combining per-field errors with errors.Join.
package validate

import (
	"errors"
)

// Sentinel causes. Each is delivered wrapped in a *FieldError.
var (
	ErrRequired      = errors.New("required")
	ErrTooLong       = errors.New("too long")
	ErrInvalidFormat = errors.New("invalid format")
	ErrOutOfRange    = errors.New("out of range")
)

// Limits used by Validate.
const (
	MaxNameLen = 50 // in runes, not bytes
	MinAge     = 13
	MaxAge     = 150
)

// FieldError reports a problem with one named field.
type FieldError struct {
	Field string // "name", "email" or "age"
	Err   error  // wraps one of the sentinels above
}

// Error returns "field: cause", e.g. "email: required".
func (e *FieldError) Error() string { return e.Field + ": " + e.Err.Error() }

// Unwrap returns the cause so errors.Is(err, ErrRequired) works.
func (e *FieldError) Unwrap() error { return e.Err }

// Signup is the form being validated.
type Signup struct {
	Name  string
	Email string
	Age   int
}

// Validate returns nil if s is valid. Otherwise it returns errors.Join of one
// *FieldError per invalid field, in the order name, email, age. Each field
// reports only its first failing rule.
//
// Rules:
//   - name: required (after trimming spaces); at most MaxNameLen runes (ErrTooLong).
//   - email: required (after trimming); must contain no whitespace, exactly
//     one '@', a non-empty part before it, and a domain after it that contains
//     a '.' but neither starts nor ends with one (ErrInvalidFormat).
//   - age: MinAge..MaxAge inclusive; the cause is
//     "out of range: must be 13-150, got N" wrapping ErrOutOfRange.
func Validate(s Signup) error {
	// TODO: Validate each field and return errors.Join of all failures.
	return errors.New("not implemented")
}

// The helpers return error, NOT *FieldError: a nil *FieldError stored in an
// error interface is non-nil, and errors.Join would keep it.

func checkName(name string) error {
	// TODO: Implement checkName.
	// Trim, check required, check max rune length.
	return errors.New("not implemented")
}

func checkEmail(email string) error {
	// TODO: Implement checkEmail.
	// Trim, check required, check format with validEmail.
	return errors.New("not implemented")
}

func validEmail(email string) bool {
	// TODO: Implement validEmail.
	// Check: no whitespace, exactly one '@', non-empty local, valid domain.
	return false
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

func checkAge(age int) error {
	// TODO: Implement checkAge.
	// Check MinAge <= age <= MaxAge, include the actual value in the message.
	return errors.New("not implemented")
}

// Fields returns the names of all fields reported in err, in order. It looks
// through any wrapping, including errors.Join trees (Unwrap() []error) and
// fmt.Errorf("...: %w", err) chains. It returns nil if err contains no
// *FieldError.
func Fields(err error) []string {
	// TODO: Implement this helper to extract field names from wrapped errors.
	// It should walk through Unwrap() []error (for errors.Join) and
	// Unwrap() error (for fmt.Errorf) chains.
	return nil
}
