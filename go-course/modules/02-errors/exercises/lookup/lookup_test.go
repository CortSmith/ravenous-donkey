package lookup

import (
	"errors"
	"fmt"
	"testing"
)

// fakeStore is a test double for Getter that records calls and returns a
// canned result.
type fakeStore struct {
	user  User
	err   error
	calls int
}

func (f *fakeStore) Get(id int) (User, error) {
	f.calls++
	return f.user, f.err
}

var users = MapStore{
	1: {ID: 1, Name: "Ada"},
	2: {ID: 2, Name: "  Grace  "},
	3: {ID: 3, Name: "   "},
}

func TestMapStoreGet(t *testing.T) {
	u, err := users.Get(1)
	if err != nil || u.Name != "Ada" {
		t.Fatalf("Get(1) = %+v, %v; want Ada, nil", u, err)
	}

	u, err = users.Get(42)
	if err == nil {
		t.Fatalf("Get(42) = %+v, nil; want error", u)
	}
	if u != (User{}) {
		t.Errorf("Get(42) returned non-zero user %+v alongside error", u)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("errors.Is(Get(42) error, ErrNotFound) = false; err = %v", err)
	}
	if err == ErrNotFound {
		t.Errorf("Get(42) returned the bare sentinel; wrap it with the ID for context")
	}
	if want := "user 42: not found"; err.Error() != want {
		t.Errorf("Get(42) error = %q, want %q", err.Error(), want)
	}
}

func TestDisplayName(t *testing.T) {
	s := &Service{Users: users}
	tests := []struct {
		id   int
		want string
	}{
		{1, "Ada"},
		{2, "Grace"},
		{3, "user #3"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.id), func(t *testing.T) {
			got, err := s.DisplayName(tt.id)
			if err != nil {
				t.Fatalf("DisplayName(%d) unexpected error: %v", tt.id, err)
			}
			if got != tt.want {
				t.Errorf("DisplayName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestDisplayNameErrors(t *testing.T) {
	errDB := errors.New("connection refused")
	tests := []struct {
		name      string
		store     *fakeStore
		id        int
		wantIs    error
		wantMsg   string
		wantCalls int
	}{
		{
			name:      "not found is wrapped with layer context",
			store:     &fakeStore{err: fmt.Errorf("user 9: %w", ErrNotFound)},
			id:        9,
			wantIs:    ErrNotFound,
			wantMsg:   "user store: user 9: not found",
			wantCalls: 1,
		},
		{
			name:      "backend failure is wrapped",
			store:     &fakeStore{err: errDB},
			id:        5,
			wantIs:    errDB,
			wantMsg:   "user store: connection refused",
			wantCalls: 1,
		},
		{
			name:      "zero id rejected without touching store",
			store:     &fakeStore{},
			id:        0,
			wantIs:    ErrInvalidID,
			wantMsg:   "user 0: invalid id",
			wantCalls: 0,
		},
		{
			name:      "negative id rejected",
			store:     &fakeStore{},
			id:        -3,
			wantIs:    ErrInvalidID,
			wantMsg:   "user -3: invalid id",
			wantCalls: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{Users: tt.store}
			got, err := s.DisplayName(tt.id)
			if err == nil {
				t.Fatalf("DisplayName(%d) = %q, nil; want error", tt.id, got)
			}
			if got != "" {
				t.Errorf("DisplayName(%d) returned %q alongside an error; want \"\"", tt.id, got)
			}
			if !errors.Is(err, tt.wantIs) {
				t.Errorf("errors.Is(err, %v) = false; err = %v", tt.wantIs, err)
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("DisplayName(%d) error = %q, want %q", tt.id, err.Error(), tt.wantMsg)
			}
			if tt.store.calls != tt.wantCalls {
				t.Errorf("store.Get called %d times, want %d", tt.store.calls, tt.wantCalls)
			}
		})
	}
}

func TestGreet(t *testing.T) {
	errDB := errors.New("connection refused")
	tests := []struct {
		name    string
		store   Getter
		id      int
		want    string
		wantErr error  // nil means success expected
		wantMsg string // checked only when wantErr != nil
	}{
		{name: "known user", store: users, id: 1, want: "Hello, Ada!"},
		{name: "blank name", store: users, id: 3, want: "Hello, user #3!"},
		{name: "missing user is a guest", store: users, id: 404, want: "Hello, guest!"},
		{
			// Any error that WRAPS ErrNotFound counts, however deep.
			name:  "deeply wrapped not found is a guest",
			store: &fakeStore{err: fmt.Errorf("cache: %w", fmt.Errorf("shard 2: %w", ErrNotFound))},
			id:    7,
			want:  "Hello, guest!",
		},
		{
			// Same text, different identity: string matching would get this wrong.
			name:    "lookalike error is not ErrNotFound",
			store:   &fakeStore{err: errors.New("not found")},
			id:      7,
			wantErr: errors.New("placeholder"),
			wantMsg: "greet: user store: not found",
		},
		{
			name:    "backend failure propagates",
			store:   &fakeStore{err: errDB},
			id:      1,
			wantErr: errDB,
			wantMsg: "greet: user store: connection refused",
		},
		{
			name:    "invalid id propagates",
			store:   users,
			id:      -1,
			wantErr: ErrInvalidID,
			wantMsg: "greet: user -1: invalid id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{Users: tt.store}
			got, err := s.Greet(tt.id)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Greet(%d) unexpected error: %v", tt.id, err)
				}
				if got != tt.want {
					t.Errorf("Greet(%d) = %q, want %q", tt.id, got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("Greet(%d) = %q, nil; want error %q", tt.id, got, tt.wantMsg)
			}
			if got != "" {
				t.Errorf("Greet(%d) returned %q alongside an error; want \"\"", tt.id, got)
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("Greet(%d) error = %q, want %q", tt.id, err.Error(), tt.wantMsg)
			}
			if tt.wantErr == errDB || tt.wantErr == ErrInvalidID {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("errors.Is(err, %v) = false; err = %v (wrap with %%w)", tt.wantErr, err)
				}
			}
			if errors.Is(err, ErrNotFound) {
				t.Errorf("Greet(%d) error %v matches ErrNotFound; it should have been handled", tt.id, err)
			}
		})
	}
}
