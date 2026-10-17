// Package hello is the first package you write in this course. Its job is to
// prove that your toolchain, editor and test runner all work, on your host or
// in the course container.
package hello

// Greeting returns "Hello, <name>!". Leading and trailing white space in name
// is ignored, and an empty or all-white-space name greets the world:
//
//	Greeting("Ada")   == "Hello, Ada!"
//	Greeting("  Ada ") == "Hello, Ada!"
//	Greeting("")      == "Hello, World!"
func Greeting(name string) string {
	// TODO: implement Greeting. The strings package has what you need.
	_ = name
	return ""
}
