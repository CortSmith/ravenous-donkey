// Package jsonio provides small, strict helpers for reading JSON request
// bodies and writing JSON responses in net/http handlers.
package jsonio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

// MaxBodyBytes is the largest request body DecodeJSON accepts.
const MaxBodyBytes = 1 << 20 // 1 MiB

// RequestError describes a problem with the client's request. Status is the
// HTTP status code that should be sent back and Msg is safe to show to the
// client.
type RequestError struct {
	Status int
	Msg    string
}

func (e *RequestError) Error() string { return e.Msg }

// WriteJSON encodes v as JSON and writes it with the given status code and a
// Content-Type of application/json. The value is encoded into a buffer first,
// so if encoding fails the client gets a 500 instead of a truncated body with
// a misleading status; the encoding error is returned.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return fmt.Errorf("encode response: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}

// WriteError writes {"error": msg} with the given status code.
func WriteError(w http.ResponseWriter, status int, msg string) {
	// Encoding a map[string]string cannot fail, and a write error means the
	// client is gone, so the error is deliberately ignored.
	_ = WriteJSON(w, status, map[string]string{"error": msg})
}

// DecodeJSON decodes the request body into dst, which must be a non-nil
// pointer. Every problem caused by the client is reported as a *RequestError
// carrying the status to send: 415 for a wrong Content-Type, 413 for a body
// larger than MaxBodyBytes and 400 for anything else (empty body, malformed
// JSON, wrong types, unknown fields, or more than one JSON value).
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// ParseMediaType lower-cases the type and strips parameters such as
	// "; charset=utf-8". An empty header is an error too.
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return &RequestError{Status: http.StatusUnsupportedMediaType, Msg: "Content-Type must be application/json"}
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return decodeError(err)
	}
	// A body such as `{"a":1}{"b":2}` would otherwise be silently accepted:
	// Decode stops after the first value. Require EOF after it.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return decodeError(err)
		}
		return &RequestError{Status: http.StatusBadRequest, Msg: "request body must contain a single JSON value"}
	}
	return nil
}

// decodeError translates json.Decoder errors into client-facing messages.
func decodeError(err error) error {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
		maxErr    *http.MaxBytesError
	)
	switch {
	case errors.As(err, &syntaxErr):
		return &RequestError{Status: http.StatusBadRequest,
			Msg: fmt.Sprintf("request body contains badly-formed JSON (at offset %d)", syntaxErr.Offset)}
	case errors.Is(err, io.ErrUnexpectedEOF):
		return &RequestError{Status: http.StatusBadRequest, Msg: "request body contains badly-formed JSON"}
	case errors.As(err, &typeErr):
		return &RequestError{Status: http.StatusBadRequest,
			Msg: fmt.Sprintf("request body contains an invalid value for field %q (at offset %d)", typeErr.Field, typeErr.Offset)}
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		// encoding/json has no typed error for this case (as of Go 1.24).
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return &RequestError{Status: http.StatusBadRequest, Msg: "request body contains unknown field " + field}
	case errors.Is(err, io.EOF):
		return &RequestError{Status: http.StatusBadRequest, Msg: "request body must not be empty"}
	case errors.As(err, &maxErr):
		return &RequestError{Status: http.StatusRequestEntityTooLarge,
			Msg: fmt.Sprintf("request body must not be larger than %d bytes", maxErr.Limit)}
	default:
		// Not the client's fault (e.g. dst is not a pointer): a programming error.
		return fmt.Errorf("decode request body: %w", err)
	}
}
