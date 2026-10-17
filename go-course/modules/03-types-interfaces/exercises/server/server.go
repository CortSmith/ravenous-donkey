// Package server practises constructors and the functional options pattern
// by building an (inert) server configuration.
package server

import (
	"errors"
	"time"
)

// Defaults applied by New before any options run.
const (
	DefaultPort         = 8080
	DefaultReadTimeout  = 5 * time.Second
	DefaultWriteTimeout = 10 * time.Second
	DefaultMaxConns     = 100
)

// ErrInvalidOption is wrapped by every error an Option returns.
var ErrInvalidOption = errors.New("invalid option")

// Server holds an immutable, validated configuration. Its fields are
// unexported so a Server can only be created through New, which guarantees
// every Server is valid.
type Server struct {
	host         string
	port         int
	readTimeout  time.Duration
	writeTimeout time.Duration
	maxConns     int
	certFile     string
	keyFile      string
}

// Option configures a Server. Options return an error so invalid values are
// reported by New instead of being silently ignored.
type Option func(*Server) error

// WithPort sets the TCP port. It must be in 1..65535; otherwise the error
// mentions "port <n>" and wraps ErrInvalidOption.
func WithPort(port int) Option {
	return func(s *Server) error { return nil } // TODO
}

// WithTimeouts sets the read and write timeouts. Both must be positive;
// otherwise the error mentions "timeout" and wraps ErrInvalidOption.
func WithTimeouts(read, write time.Duration) Option {
	return func(s *Server) error { return nil } // TODO
}

// WithMaxConns limits the number of simultaneous connections. n must be at
// least 1; otherwise the error mentions "max conns" and wraps
// ErrInvalidOption.
func WithMaxConns(n int) Option {
	return func(s *Server) error { return nil } // TODO
}

// WithTLS enables TLS using the given certificate and key files. Both paths
// must be non-empty; otherwise the error mentions "TLS" and wraps
// ErrInvalidOption.
func WithTLS(certFile, keyFile string) Option {
	return func(s *Server) error { return nil } // TODO
}

// New returns a Server listening on host (which may be empty, meaning all
// interfaces) with the defaults above, modified by opts in order (so a
// later option overrides an earlier one). nil options are skipped. The
// first failing option aborts construction: New returns nil and that error
// wrapped with the "server: " prefix.
func New(host string, opts ...Option) (*Server, error) {
	return nil, errors.New("not implemented") // TODO
}

// Addr returns the "host:port" address, bracketing IPv6 hosts as needed
// ("[::1]:8080"). Hint: the net package already knows how.
func (s *Server) Addr() string { return "" } // TODO

// ReadTimeout returns the configured read timeout.
func (s *Server) ReadTimeout() time.Duration { return 0 } // TODO

// WriteTimeout returns the configured write timeout.
func (s *Server) WriteTimeout() time.Duration { return 0 } // TODO

// MaxConns returns the connection limit.
func (s *Server) MaxConns() int { return 0 } // TODO

// TLS reports whether TLS is enabled.
func (s *Server) TLS() bool { return false } // TODO

// String summarises the configuration, e.g.
// "http://localhost:8080 (read 5s, write 10s, max 100 conns)", using
// "https" when TLS is enabled.
func (s *Server) String() string { return "" } // TODO
