// Package config loads and validates a JSON service configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// ErrInvalid is wrapped by every validation error returned by Validate.
var ErrInvalid = errors.New("invalid config")

// Duration is a time.Duration that is written in JSON as a string such as
// "1m30s" (see time.ParseDuration), instead of as an opaque nanosecond count.
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"30s\": %w", err)
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// Config is the service configuration.
type Config struct {
	Name     string   `json:"name"`           // required, not blank
	Listen   string   `json:"listen"`         // host:port, port 1-65535; default ":8080"
	Timeout  Duration `json:"timeout"`        // > 0; default 30s
	MaxConns int      `json:"max_conns"`      // 1-10000; default 100
	Tags     []string `json:"tags,omitempty"` // optional; no empty tags
	Debug    bool     `json:"debug"`          // default false
}

// Default returns a Config holding the default value of every field. Name
// has no default.
func Default() Config {
	return Config{
		Listen:   ":8080",
		Timeout:  Duration(30 * time.Second),
		MaxConns: 100,
	}
}

// Load decodes exactly one JSON object from r on top of Default() and then
// validates it. Fields absent from the JSON keep their defaults, but a field
// that is present always wins, even if it holds a zero value (so
// "max_conns": 0 is rejected rather than silently replaced by 100).
//
// Load rejects unknown fields, values of the wrong type, and anything other
// than whitespace after the object.
func Load(r io.Reader) (Config, error) {
	cfg := Default()
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("config: decoding: %w", err)
	}
	// Decode stops after the first value. Anything else must be EOF.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Config{}, errors.New("config: unexpected data after the JSON object")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// LoadFile opens the named file and calls Load. Errors mention the path.
func LoadFile(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err // *fs.PathError already includes the path
	}
	defer f.Close() // read-only: a Close error can't lose data

	cfg, err := Load(f)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Validate reports every problem with c, not just the first, as errors
// joined with errors.Join. Each one wraps ErrInvalid and names the JSON
// field. It returns nil if c is valid.
func (c Config) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, fmt.Errorf("%w: name is required", ErrInvalid))
	}
	if _, port, err := net.SplitHostPort(c.Listen); err != nil {
		errs = append(errs, fmt.Errorf("%w: listen %q: %v", ErrInvalid, c.Listen, err))
	} else if p, err := strconv.Atoi(port); err != nil || p < 1 || p > 65535 {
		errs = append(errs, fmt.Errorf("%w: listen %q: port must be 1-65535", ErrInvalid, c.Listen))
	}
	if c.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("%w: timeout must be positive, got %v", ErrInvalid, time.Duration(c.Timeout)))
	}
	if c.MaxConns < 1 || c.MaxConns > 10000 {
		errs = append(errs, fmt.Errorf("%w: max_conns must be 1-10000, got %d", ErrInvalid, c.MaxConns))
	}
	for i, tag := range c.Tags {
		if tag == "" {
			errs = append(errs, fmt.Errorf("%w: tags[%d] is empty", ErrInvalid, i))
		}
	}
	return errors.Join(errs...) // nil when errs is empty
}
