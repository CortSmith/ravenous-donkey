// Package validate checks a sign-up form and reports EVERY problem at once,
// combining per-field errors with errors.Join.
package validate

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
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
	return errors.Join(
		checkName(s.Name),
		checkEmail(s.Email),
		checkAge(s.Age),
	)
}

// The helpers return error, NOT *FieldError: a nil *FieldError stored in an
// error interface is non-nil, and errors.Join would keep it.

func checkName(name string) error {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return &FieldError{Field: "name", Err: ErrRequired}
	case utf8.RuneCountInString(name) > MaxNameLen:
		return &FieldError{Field: "name", Err: ErrTooLong}
	}
	return nil
}

func checkEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return &FieldError{Field: "email", Err: ErrRequired}
	}
	if !validEmail(email) {
		return &FieldError{Field: "email", Err: ErrInvalidFormat}
	}
	return nil
}

func validEmail(email string) bool {
	if strings.ContainsFunc(email, isSpace) {
		return false
	}
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || strings.Contains(domain, "@") {
		return false
	}
	return strings.Contains(domain, ".") &&
		!strings.HasPrefix(domain, ".") &&
		!strings.HasSuffix(domain, ".")
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

func checkAge(age int) error {
	if age < MinAge || age > MaxAge {
		return &FieldError{
			Field: "age",
			Err:   fmt.Errorf("%w: must be %d-%d, got %d", ErrOutOfRange, MinAge, MaxAge, age),
		}
	}
	return nil
}

// Fields returns the names of all fields reported in err, in order. It looks
// through any wrapping, including errors.Join trees (Unwrap() []error) and
// fmt.Errorf("...: %w", err) chains. It returns nil if err contains no
// *FieldError.
func Fields(err error) []string {
	var out []string
	var walk func(error)
	walk = func(e error) {
		switch x := e.(type) {
		case nil:
			return
		case *FieldError:
			out = append(out, x.Field)
		case interface{ Unwrap() []error }:
			for _, inner := range x.Unwrap() {
				walk(inner)
			}
		case interface{ Unwrap() error }:
			walk(x.Unwrap())
		}
	}
	walk(err)
	return out
}
