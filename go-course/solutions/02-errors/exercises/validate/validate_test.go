package validate

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

var valid = Signup{Name: "Ada Lovelace", Email: "ada@example.com", Age: 36}

func TestValidateOK(t *testing.T) {
	tests := []struct {
		name string
		in   Signup
	}{
		{"typical", valid},
		{"min age", Signup{Name: "A", Email: "a@b.co", Age: MinAge}},
		{"max age", Signup{Name: "A", Email: "a@b.co", Age: MaxAge}},
		{"name exactly max runes, multibyte", Signup{Name: strings.Repeat("é", MaxNameLen), Email: "x@y.z", Age: 20}},
		{"surrounding spaces trimmed", Signup{Name: "  Bob ", Email: " bob@mail.example.org  ", Age: 20}},
		{"subdomains and plus", Signup{Name: "C", Email: "c+tag@mail.example.co.uk", Age: 20}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compare with == nil on purpose: a nil *FieldError wrapped in an
			// error interface is NOT nil, and this catches that bug.
			if err := Validate(tt.in); err != nil {
				t.Errorf("Validate(%+v) = %v (type %T), want nil", tt.in, err, err)
			}
		})
	}
}

func TestValidateSingleField(t *testing.T) {
	tests := []struct {
		name      string
		in        Signup
		wantField string
		wantIs    error
		wantMsg   string
	}{
		{"name empty", Signup{Name: "", Email: valid.Email, Age: 30}, "name", ErrRequired, "name: required"},
		{"name blank", Signup{Name: " \t ", Email: valid.Email, Age: 30}, "name", ErrRequired, "name: required"},
		{"name too long", Signup{Name: strings.Repeat("é", MaxNameLen+1), Email: valid.Email, Age: 30}, "name", ErrTooLong, "name: too long"},
		{"email empty", Signup{Name: "A", Email: "  ", Age: 30}, "email", ErrRequired, "email: required"},
		{"email no at", Signup{Name: "A", Email: "ada.example.com", Age: 30}, "email", ErrInvalidFormat, "email: invalid format"},
		{"email two ats", Signup{Name: "A", Email: "a@b@c.com", Age: 30}, "email", ErrInvalidFormat, "email: invalid format"},
		{"email empty local", Signup{Name: "A", Email: "@example.com", Age: 30}, "email", ErrInvalidFormat, "email: invalid format"},
		{"email no dot in domain", Signup{Name: "A", Email: "a@localhost", Age: 30}, "email", ErrInvalidFormat, "email: invalid format"},
		{"email domain leading dot", Signup{Name: "A", Email: "a@.example.com", Age: 30}, "email", ErrInvalidFormat, "email: invalid format"},
		{"email domain trailing dot", Signup{Name: "A", Email: "a@example.", Age: 30}, "email", ErrInvalidFormat, "email: invalid format"},
		{"email inner space", Signup{Name: "A", Email: "a b@example.com", Age: 30}, "email", ErrInvalidFormat, "email: invalid format"},
		{"age too young", Signup{Name: "A", Email: valid.Email, Age: MinAge - 1}, "age", ErrOutOfRange, "age: out of range: must be 13-150, got 12"},
		{"age too old", Signup{Name: "A", Email: valid.Email, Age: MaxAge + 1}, "age", ErrOutOfRange, "age: out of range: must be 13-150, got 151"},
		{"age negative", Signup{Name: "A", Email: valid.Email, Age: -5}, "age", ErrOutOfRange, "age: out of range: must be 13-150, got -5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.in)
			if err == nil {
				t.Fatalf("Validate(%+v) = nil, want error", tt.in)
			}
			if !errors.Is(err, tt.wantIs) {
				t.Errorf("errors.Is(err, %v) = false; err = %v", tt.wantIs, err)
			}
			var fe *FieldError
			if !errors.As(err, &fe) {
				t.Fatalf("errors.As(err, *FieldError) = false; err = %v (%T)", err, err)
			}
			if fe.Field != tt.wantField {
				t.Errorf("FieldError.Field = %q, want %q", fe.Field, tt.wantField)
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("Validate(%+v).Error() = %q, want %q", tt.in, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestValidateReportsAllFields(t *testing.T) {
	in := Signup{Name: "", Email: "nope", Age: 200}
	err := Validate(in)
	if err == nil {
		t.Fatalf("Validate(%+v) = nil, want error", in)
	}

	// errors.Join produces an error with Unwrap() []error.
	multi, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("Validate error %T does not implement Unwrap() []error; use errors.Join", err)
	}
	if n := len(multi.Unwrap()); n != 3 {
		t.Errorf("joined error has %d children, want 3: %v", n, multi.Unwrap())
	}

	for _, sentinel := range []error{ErrRequired, ErrInvalidFormat, ErrOutOfRange} {
		if !errors.Is(err, sentinel) {
			t.Errorf("errors.Is(err, %v) = false; err = %v", sentinel, err)
		}
	}
	if errors.Is(err, ErrTooLong) {
		t.Errorf("errors.Is(err, ErrTooLong) = true; no field is too long")
	}

	want := "name: required\nemail: invalid format\nage: out of range: must be 13-150, got 200"
	if err.Error() != want {
		t.Errorf("Validate(%+v).Error() =\n%s\nwant\n%s", in, err.Error(), want)
	}
}

func TestFields(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want []string
	}{
		{"nil", nil, nil},
		{"unrelated error", errors.New("boom"), nil},
		{"valid input", Validate(valid), nil},
		{"one field", Validate(Signup{Name: "A", Email: "", Age: 30}), []string{"email"}},
		{"all fields in order", Validate(Signup{}), []string{"name", "email", "age"}},
		{"wrapped joined error", fmt.Errorf("signup: %w", Validate(Signup{Name: "A", Age: 1})), []string{"email", "age"}},
		{"direct FieldError", &FieldError{Field: "zip", Err: ErrRequired}, []string{"zip"}},
		{
			"nested joins",
			errors.Join(&FieldError{Field: "a", Err: ErrRequired}, errors.Join(errors.New("x"), &FieldError{Field: "b", Err: ErrTooLong})),
			[]string{"a", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Fields(tt.err)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Fields(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestFieldErrorMethods(t *testing.T) {
	fe := &FieldError{Field: "zip", Err: ErrRequired}
	if got, want := fe.Error(), "zip: required"; got != want {
		t.Errorf("FieldError.Error() = %q, want %q", got, want)
	}
	if fe.Unwrap() != ErrRequired {
		t.Errorf("FieldError.Unwrap() = %v, want ErrRequired", fe.Unwrap())
	}
}
