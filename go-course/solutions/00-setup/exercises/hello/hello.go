// Package hello is the first package you write in this course. Its job is to
// prove that your toolchain, editor and test runner all work, on your host or
// in the course container.
package hello

import "strings"

// Greeting returns "Hello, <name>!". Leading and trailing white space in name
// is ignored, and an empty or all-white-space name greets the world:
//
//	Greeting("Ada")   == "Hello, Ada!"
//	Greeting("  Ada ") == "Hello, Ada!"
//	Greeting("")      == "Hello, World!"
func Greeting(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "World"
	}
	return "Hello, " + name + "!"
}
