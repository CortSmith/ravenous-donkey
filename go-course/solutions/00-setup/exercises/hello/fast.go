package hello

import (
	"fmt"
	"strings"
)

// GreetingSprintf is the version most people write first. It is just as
// readable as Greeting but goes through fmt's reflection-based formatting.
func GreetingSprintf(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "World"
	}
	return fmt.Sprintf("Hello, %s!", name)
}

// GreetingBuilder pre-sizes a strings.Builder. For three short pieces it is
// only ~10% faster than +, which the compiler already turns into a single
// allocation: not worth the extra lines.
func GreetingBuilder(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "World"
	}
	var b strings.Builder
	b.Grow(len("Hello, !") + len(name))
	b.WriteString("Hello, ")
	b.WriteString(name)
	b.WriteByte('!')
	return b.String()
}
