package hello

import (
	"fmt"
	"testing"
)

func TestGreeting(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain name", "Ada", "Hello, Ada!"},
		{"surrounding spaces", "  Ada ", "Hello, Ada!"},
		{"tabs and newline", "\tGo\n", "Hello, Go!"},
		{"inner space kept", "Grace Hopper", "Hello, Grace Hopper!"},
		{"empty", "", "Hello, World!"},
		{"only white space", " \t ", "Hello, World!"},
		{"unicode", "Gödel", "Hello, Gödel!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Greeting(tt.in); got != tt.want {
				t.Errorf("Greeting(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func ExampleGreeting() {
	fmt.Println(Greeting("Gopher"))
	fmt.Println(Greeting(""))
	// Output:
	// Hello, Gopher!
	// Hello, World!
}
