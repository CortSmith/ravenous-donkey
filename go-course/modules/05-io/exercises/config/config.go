// Package config loads and validates a JSON service configuration.
package config

import (
	"errors"
	"io"
	"time"
)

// ErrInvalid is wrapped by every validation error returned by Validate.
var ErrInvalid = errors.New("invalid config")

// Duration is a time.Duration that is written in JSON as a string such as
// "1m30s" (see time.ParseDuration), instead of as an opaque nanosecond count.
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler.
//
// TODO: decode b as a JSON string (a JSON number must be rejected with an
// error that mentions "duration"), then parse it with time.ParseDuration.
func (d *Duration) UnmarshalJSON(b []byte) error {
	return errors.New("not implemented")
}

// MarshalJSON implements json.Marshaler.
//
// TODO: encode d as a JSON string using time.Duration's String method.
func (d Duration) MarshalJSON() ([]byte, error) {
	return nil, errors.New("not implemented")
}

// Config is the service configuration.
//
// TODO: add json struct tags. The JSON keys are "name", "listen", "timeout",
// "max_conns", "tags" (omitted when empty) and "debug".
type Config struct {
	Name     string   // required, not blank
	Listen   string   // host:port, port 1-65535; default ":8080"
	Timeout  Duration // > 0; default 30s
	MaxConns int      // 1-10000; default 100
	Tags     []string // optional; no empty tags
	Debug    bool     // default false
}

// Default returns a Config holding the default value of every field. Name
// has no default.
func Default() Config {
	return Config{} // TODO
}

// Load decodes exactly one JSON object from r on top of Default() and then
// validates it. Fields absent from the JSON keep their defaults, but a field
// that is present always wins, even if it holds a zero value (so
// "max_conns": 0 is rejected rather than silently replaced by 100).
//
// Load rejects unknown fields, values of the wrong type, and anything other
// than whitespace after the object.
//
// TODO: use a json.Decoder with DisallowUnknownFields. Decode reads only the
// first value: check that nothing follows it. Wrap decoding errors with %w
// so callers can still use errors.As(err, new(*json.SyntaxError)).
func Load(r io.Reader) (Config, error) {
	return Config{}, errors.New("not implemented")
}

// LoadFile opens the named file and calls Load. Errors mention the path, and
// a missing file satisfies errors.Is(err, fs.ErrNotExist).
//
// TODO: implement. Don't forget to close the file.
func LoadFile(path string) (Config, error) {
	return Config{}, errors.New("not implemented")
}

// Validate reports every problem with c, not just the first, as errors
// joined with errors.Join. Each one wraps ErrInvalid and names the JSON
// field ("name", "listen", "timeout", "max_conns", or "tags[i]" for the
// i-th tag). It returns nil if c is valid.
//
// TODO: implement. Use net.SplitHostPort for Listen.
func (c Config) Validate() error {
	return nil
}
