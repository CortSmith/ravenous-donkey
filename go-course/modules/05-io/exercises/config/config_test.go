package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	want := Config{Listen: ":8080", Timeout: Duration(30 * time.Second), MaxConns: 100}
	if got := Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("Default() = %+v, want %+v", got, want)
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Config
	}{
		{
			name: "only required field uses defaults",
			in:   `{"name": "svc"}`,
			want: Config{Name: "svc", Listen: ":8080", Timeout: Duration(30 * time.Second), MaxConns: 100},
		},
		{
			name: "everything set",
			in:   `{"name":"svc","listen":"0.0.0.0:443","timeout":"250ms","max_conns":1,"tags":["a","b"],"debug":true}`,
			want: Config{Name: "svc", Listen: "0.0.0.0:443", Timeout: Duration(250 * time.Millisecond), MaxConns: 1, Tags: []string{"a", "b"}, Debug: true},
		},
		{
			name: "surrounding whitespace is fine",
			in:   "\n\t {\"name\": \"svc\", \"max_conns\": 10000}  \n\n",
			want: Config{Name: "svc", Listen: ":8080", Timeout: Duration(30 * time.Second), MaxConns: 10000},
		},
		{
			name: "ipv6 listen address",
			in:   `{"name": "svc", "listen": "[::1]:65535"}`,
			want: Config{Name: "svc", Listen: "[::1]:65535", Timeout: Duration(30 * time.Second), MaxConns: 100},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("Load(%s) error = %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Load(%s)\n got %+v\nwant %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		contains []string // substrings the error message must contain
	}{
		{"empty input", ``, nil},
		{"not json", `name = "svc"`, nil},
		{"truncated", `{"name": "svc"`, nil},
		{"unknown field", `{"name": "svc", "max_con": 5}`, []string{"max_con"}},
		{"wrong type", `{"name": "svc", "max_conns": "many"}`, []string{"max_conns"}},
		{"numeric timeout", `{"name": "svc", "timeout": 30}`, []string{"duration"}},
		{"bad duration", `{"name": "svc", "timeout": "soon"}`, []string{"soon"}},
		{"trailing object", `{"name": "a"} {"name": "b"}`, nil},
		{"trailing garbage", `{"name": "a"} x`, nil},
		{"trailing brace", `{"name": "a"}}`, nil},
		{"array not object", `[{"name": "a"}]`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(strings.NewReader(tt.in))
			if err == nil {
				t.Fatalf("Load(%s) error = nil, want an error", tt.in)
			}
			for _, s := range tt.contains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("Load(%s) error = %q, want it to mention %q", tt.in, err, s)
				}
			}
		})
	}
}

func TestLoadPreservesJSONErrorTypes(t *testing.T) {
	_, err := Load(strings.NewReader(`{"name": 42}`))
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) {
		t.Errorf("Load with wrong type: error = %v, want it to wrap *json.UnmarshalTypeError", err)
	}
	_, err = Load(strings.NewReader(`{"name": "x",}`))
	var synErr *json.SyntaxError
	if !errors.As(err, &synErr) {
		t.Errorf("Load with bad syntax: error = %v, want it to wrap *json.SyntaxError", err)
	}
}

func TestValidate(t *testing.T) {
	valid := func() Config {
		c := Default()
		c.Name = "svc"
		return c
	}
	tests := []struct {
		name   string
		modify func(*Config)
		want   []string // one substring per expected problem; empty means valid
	}{
		{"valid", func(c *Config) {}, nil},
		{"blank name", func(c *Config) { c.Name = "  " }, []string{"name"}},
		{"listen without port", func(c *Config) { c.Listen = "localhost" }, []string{"listen"}},
		{"listen port zero", func(c *Config) { c.Listen = ":0" }, []string{"listen"}},
		{"listen port too big", func(c *Config) { c.Listen = ":65536" }, []string{"listen"}},
		{"listen port not numeric", func(c *Config) { c.Listen = "host:http" }, []string{"listen"}},
		{"zero timeout", func(c *Config) { c.Timeout = 0 }, []string{"timeout"}},
		{"negative timeout", func(c *Config) { c.Timeout = Duration(-time.Second) }, []string{"timeout"}},
		{"max_conns zero", func(c *Config) { c.MaxConns = 0 }, []string{"max_conns"}},
		{"max_conns too big", func(c *Config) { c.MaxConns = 10001 }, []string{"max_conns"}},
		{"empty tag", func(c *Config) { c.Tags = []string{"a", ""} }, []string{"tags[1]"}},
		{"several problems at once", func(c *Config) {
			c.Name = ""
			c.MaxConns = -1
			c.Timeout = 0
		}, []string{"name", "max_conns", "timeout"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.modify(&c)
			err := c.Validate()
			if len(tt.want) == 0 {
				if err != nil {
					t.Fatalf("Validate(%+v) = %v, want nil", c, err)
				}
				return
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Validate(%+v) = %v, want errors.Is(err, ErrInvalid)", c, err)
			}
			for _, s := range tt.want {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("Validate error %q does not mention %q", err, s)
				}
			}
			if lines := strings.Count(err.Error(), "\n") + 1; lines != len(tt.want) {
				t.Errorf("Validate reported %d problem(s), want %d (use errors.Join, one error per problem):\n%v", lines, len(tt.want), err)
			}
		})
	}
}

func TestExplicitZeroIsNotDefaulted(t *testing.T) {
	_, err := Load(strings.NewReader(`{"name": "svc", "max_conns": 0}`))
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "max_conns") {
		t.Errorf(`Load with "max_conns": 0: error = %v, want an ErrInvalid about max_conns (explicit zero must not be replaced by the default)`, err)
	}
}

func TestDurationJSONRoundTrip(t *testing.T) {
	cfg := Default()
	cfg.Name = "round-trip"
	cfg.Timeout = Duration(90 * time.Second)
	cfg.Tags = []string{"x"}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	if !strings.Contains(string(b), `"timeout":"1m30s"`) {
		t.Errorf("json.Marshal = %s, want timeout encoded as \"1m30s\"", b)
	}
	got, err := Load(strings.NewReader(string(b)))
	if err != nil {
		t.Fatalf("Load(Marshal(cfg)) error = %v", err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Errorf("round trip:\n got %+v\nwant %+v", got, cfg)
	}
}

func TestLoadFile(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := LoadFile(filepath.Join("testdata", "valid.json"))
		if err != nil {
			t.Fatalf("LoadFile error = %v", err)
		}
		want := Config{
			Name: "inventory-api", Listen: "127.0.0.1:9090", Timeout: Duration(90 * time.Second),
			MaxConns: 250, Tags: []string{"prod", "eu-west"}, Debug: true,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LoadFile(valid.json)\n got %+v\nwant %+v", got, want)
		}
	})
	t.Run("minimal", func(t *testing.T) {
		got, err := LoadFile(filepath.Join("testdata", "minimal.json"))
		if err != nil {
			t.Fatalf("LoadFile error = %v", err)
		}
		want := Default()
		want.Name = "tiny"
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LoadFile(minimal.json)\n got %+v\nwant %+v", got, want)
		}
	})

	errCases := []struct {
		file     string
		contains []string
		isErr    error
	}{
		{"does-not-exist.json", []string{"does-not-exist.json"}, fs.ErrNotExist},
		{"unknown_field.json", []string{"unknown_field.json", "max_con"}, nil},
		{"invalid_values.json", []string{"invalid_values.json", "name", "listen", "timeout", "max_conns", "tags[1]"}, ErrInvalid},
		{"trailing.json", []string{"trailing.json"}, nil},
	}
	for _, tt := range errCases {
		t.Run(tt.file, func(t *testing.T) {
			_, err := LoadFile(filepath.Join("testdata", tt.file))
			if err == nil {
				t.Fatalf("LoadFile(%s) error = nil, want an error", tt.file)
			}
			if tt.isErr != nil && !errors.Is(err, tt.isErr) {
				t.Errorf("LoadFile(%s) error = %v, want errors.Is(err, %v)", tt.file, err, tt.isErr)
			}
			for _, s := range tt.contains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("LoadFile(%s) error = %q, want it to mention %q", tt.file, err, s)
				}
			}
		})
	}
}
