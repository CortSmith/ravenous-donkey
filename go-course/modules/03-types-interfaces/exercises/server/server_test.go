package server

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	s, err := New("localhost")
	if err != nil {
		t.Fatalf("New(\"localhost\") error = %v, want nil", err)
	}
	if s == nil {
		t.Fatal("New returned a nil *Server with a nil error")
	}
	if got, want := s.Addr(), "localhost:8080"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
	if got := s.ReadTimeout(); got != DefaultReadTimeout {
		t.Errorf("ReadTimeout() = %v, want %v", got, DefaultReadTimeout)
	}
	if got := s.WriteTimeout(); got != DefaultWriteTimeout {
		t.Errorf("WriteTimeout() = %v, want %v", got, DefaultWriteTimeout)
	}
	if got := s.MaxConns(); got != DefaultMaxConns {
		t.Errorf("MaxConns() = %d, want %d", got, DefaultMaxConns)
	}
	if s.TLS() {
		t.Error("TLS() = true by default, want false")
	}
}

func TestOptions(t *testing.T) {
	s, err := New("example.com",
		WithPort(9443),
		WithTimeouts(time.Second, 2*time.Second),
		WithMaxConns(5),
		WithTLS("cert.pem", "key.pem"),
	)
	if err != nil {
		t.Fatalf("New with valid options error = %v", err)
	}
	if got, want := s.Addr(), "example.com:9443"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
	if got := s.ReadTimeout(); got != time.Second {
		t.Errorf("ReadTimeout() = %v, want 1s", got)
	}
	if got := s.WriteTimeout(); got != 2*time.Second {
		t.Errorf("WriteTimeout() = %v, want 2s", got)
	}
	if got := s.MaxConns(); got != 5 {
		t.Errorf("MaxConns() = %d, want 5", got)
	}
	if !s.TLS() {
		t.Error("TLS() = false after WithTLS, want true")
	}
	if got, want := s.String(), "https://example.com:9443 (read 1s, write 2s, max 5 conns)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestAddr(t *testing.T) {
	tests := []struct {
		host string
		port int
		want string
	}{
		{"localhost", 80, "localhost:80"},
		{"", 8080, ":8080"},
		{"127.0.0.1", 1, "127.0.0.1:1"},
		{"::1", 8080, "[::1]:8080"},
		{"fe80::1", 65535, "[fe80::1]:65535"},
	}
	for _, tt := range tests {
		s, err := New(tt.host, WithPort(tt.port))
		if err != nil {
			t.Errorf("New(%q, WithPort(%d)) error = %v", tt.host, tt.port, err)
			continue
		}
		if got := s.Addr(); got != tt.want {
			t.Errorf("New(%q, WithPort(%d)).Addr() = %q, want %q", tt.host, tt.port, got, tt.want)
		}
	}
}

func TestLaterOptionWins(t *testing.T) {
	s, err := New("h", WithPort(1000), WithMaxConns(1), WithPort(2000), WithMaxConns(2))
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	if got, want := s.Addr(), "h:2000"; got != want {
		t.Errorf("Addr() = %q, want %q (options must apply in order)", got, want)
	}
	if got := s.MaxConns(); got != 2 {
		t.Errorf("MaxConns() = %d, want 2", got)
	}
}

func TestNilOptionIgnored(t *testing.T) {
	s, err := New("h", nil, WithPort(1234), nil)
	if err != nil {
		t.Fatalf("New with nil options error = %v", err)
	}
	if got, want := s.Addr(), "h:1234"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
}

func TestString(t *testing.T) {
	s, err := New("localhost")
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	if got, want := s.String(), "http://localhost:8080 (read 5s, write 10s, max 100 conns)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestInvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		opt     Option
		wantMsg string // substring the error must contain
	}{
		{"port zero", WithPort(0), "port 0"},
		{"port negative", WithPort(-1), "port -1"},
		{"port too big", WithPort(65536), "port 65536"},
		{"zero read timeout", WithTimeouts(0, time.Second), "timeout"},
		{"negative write timeout", WithTimeouts(time.Second, -time.Second), "timeout"},
		{"zero max conns", WithMaxConns(0), "max conns"},
		{"tls without key", WithTLS("cert.pem", ""), "TLS"},
		{"tls without cert", WithTLS("", "key.pem"), "TLS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New("h", tt.opt)
			if err == nil {
				t.Fatalf("New(%s) = %v, nil; want an error", tt.name, s)
			}
			if s != nil {
				t.Errorf("New(%s) returned a non-nil *Server together with an error", tt.name)
			}
			if !errors.Is(err, ErrInvalidOption) {
				t.Errorf("error %q does not wrap ErrInvalidOption", err)
			}
			if !strings.HasPrefix(err.Error(), "server: ") {
				t.Errorf("error %q should start with \"server: \"", err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error %q should mention %q", err, tt.wantMsg)
			}
		})
	}
}

func TestFirstErrorStops(t *testing.T) {
	ran := false
	spy := Option(func(*Server) error { ran = true; return nil })
	if _, err := New("h", WithPort(0), spy); err == nil {
		t.Fatal("New with an invalid option returned nil error")
	}
	if ran {
		t.Error("options after a failing option were still applied")
	}
}

func TestCustomOption(t *testing.T) {
	// Option is a plain function type, so any func(*Server) error can be
	// passed as one. (Outside this package the fields are unexported, so
	// callers can only compose the provided options.)
	custom := func(s *Server) error { s.maxConns = 42; return nil }
	s, err := New("h", custom)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	if got := s.MaxConns(); got != 42 {
		t.Errorf("MaxConns() = %d, want 42", got)
	}
}
