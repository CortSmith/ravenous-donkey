// Package jsonio provides small, strict helpers for reading JSON request
// bodies and writing JSON responses in net/http handlers.
package jsonio

import (
	"net/http"
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
	// TODO: implement WriteJSON
	_ = w
	_ = status
	_ = v
	return nil
}

// WriteError writes {"error": msg} with the given status code.
func WriteError(w http.ResponseWriter, status int, msg string) {
	// TODO: implement WriteError
	_ = w
	_ = status
	_ = msg
}

// DecodeJSON decodes the request body into dst, which must be a non-nil
// pointer. Every problem caused by the client is reported as a *RequestError
// carrying the status to send: 415 for a wrong Content-Type, 413 for a body
// larger than MaxBodyBytes and 400 for anything else (empty body, malformed
// JSON, wrong types, unknown fields, or more than one JSON value).
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// TODO: implement DecodeJSON
	_ = w
	_ = r
	_ = dst
	return nil
}
