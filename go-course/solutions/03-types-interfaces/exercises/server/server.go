// Package server practises constructors and the functional options pattern
// by building an (inert) server configuration.
package server

import (
	"errors"
	"fmt"
	"net"
	"strconv"
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

// WithPort sets the TCP port. It must be in 1..65535.
func WithPort(port int) Option {
	return func(s *Server) error {
		if port < 1 || port > 65535 {
			return fmt.Errorf("port %d out of range 1-65535: %w", port, ErrInvalidOption)
		}
		s.port = port
		return nil
	}
}

// WithTimeouts sets the read and write timeouts. Both must be positive.
func WithTimeouts(read, write time.Duration) Option {
	return func(s *Server) error {
		if read <= 0 || write <= 0 {
			return fmt.Errorf("timeouts must be positive (read=%v, write=%v): %w", read, write, ErrInvalidOption)
		}
		s.readTimeout, s.writeTimeout = read, write
		return nil
	}
}

// WithMaxConns limits the number of simultaneous connections. n must be at
// least 1.
func WithMaxConns(n int) Option {
	return func(s *Server) error {
		if n < 1 {
			return fmt.Errorf("max conns %d must be at least 1: %w", n, ErrInvalidOption)
		}
		s.maxConns = n
		return nil
	}
}

// WithTLS enables TLS using the given certificate and key files. Both paths
// must be non-empty.
func WithTLS(certFile, keyFile string) Option {
	return func(s *Server) error {
		if certFile == "" || keyFile == "" {
			return fmt.Errorf("TLS needs both a cert file and a key file: %w", ErrInvalidOption)
		}
		s.certFile, s.keyFile = certFile, keyFile
		return nil
	}
}

// New returns a Server listening on host (which may be empty, meaning all
// interfaces) with the defaults above, modified by opts in order (so a
// later option overrides an earlier one). nil options are skipped. The
// first failing option aborts construction; its error is returned wrapped
// with the "server: " prefix.
func New(host string, opts ...Option) (*Server, error) {
	s := &Server{
		host:         host,
		port:         DefaultPort,
		readTimeout:  DefaultReadTimeout,
		writeTimeout: DefaultWriteTimeout,
		maxConns:     DefaultMaxConns,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("server: %w", err)
		}
	}
	return s, nil
}

// Addr returns the "host:port" address, bracketing IPv6 hosts as needed.
func (s *Server) Addr() string { return net.JoinHostPort(s.host, strconv.Itoa(s.port)) }

// ReadTimeout returns the configured read timeout.
func (s *Server) ReadTimeout() time.Duration { return s.readTimeout }

// WriteTimeout returns the configured write timeout.
func (s *Server) WriteTimeout() time.Duration { return s.writeTimeout }

// MaxConns returns the connection limit.
func (s *Server) MaxConns() int { return s.maxConns }

// TLS reports whether TLS is enabled.
func (s *Server) TLS() bool { return s.certFile != "" }

// String summarises the configuration, e.g.
// "http://localhost:8080 (read 5s, write 10s, max 100 conns)".
func (s *Server) String() string {
	scheme := "http"
	if s.TLS() {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s (read %v, write %v, max %d conns)",
		scheme, s.Addr(), s.readTimeout, s.writeTimeout, s.maxConns)
}
