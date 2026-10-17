package validate

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Preallocated errors for causes that carry no dynamic data. Returning the
// same pointer every time costs zero allocations.
//
// Caveat: these are shared, mutable pointers. A caller that does
// errors.As(err, &fe); fe.Field = "x" would corrupt every future error.
// That is why the readable version builds a fresh *FieldError each time.
var (
	errNameRequired = &FieldError{Field: "name", Err: ErrRequired}
	errNameTooLong  = &FieldError{Field: "name", Err: ErrTooLong}
	errEmailReq     = &FieldError{Field: "email", Err: ErrRequired}
	errEmailFormat  = &FieldError{Field: "email", Err: ErrInvalidFormat}
)

// ValidateFast is Validate with fewer allocations:
//
//   - static field errors are preallocated (see above);
//   - the valid path returns nil before touching errors.Join;
//   - name length uses a byte-length shortcut: a string of at most
//     MaxNameLen bytes cannot have more than MaxNameLen runes, so
//     RuneCountInString runs only for long names.
//
// The age error still allocates, because its message includes the value.
func ValidateFast(s Signup) error {
	nameErr := checkNameFast(s.Name)
	emailErr := checkEmailFast(s.Email)
	ageErr := checkAge(s.Age)
	if nameErr == nil && emailErr == nil && ageErr == nil {
		return nil
	}
	return errors.Join(nameErr, emailErr, ageErr)
}

func checkNameFast(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errNameRequired
	}
	if len(name) > MaxNameLen && utf8.RuneCountInString(name) > MaxNameLen {
		return errNameTooLong
	}
	return nil
}

func checkEmailFast(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errEmailReq
	}
	if !validEmail(email) {
		return errEmailFormat
	}
	return nil
}
