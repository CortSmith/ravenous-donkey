// Package describe practises type switches, type assertions and the
// "nil interface vs nil pointer" gotcha.
package describe

import "strconv"

// Describe returns a short human-readable description of v that includes
// its type. The rules, in priority order:
//
//	nil (untyped)                    -> "nil"
//	error                            -> "error: " + v.Error()
//	fmt.Stringer                     -> "Stringer: " + v.String()
//	bool                             -> "bool true"
//	int, int8..int64, uint..uint64   -> "<type> <value>", e.g. "int8 -3"
//	float32, float64                 -> "<type> <value>" using the shortest
//	                                    representation, e.g. "float64 2.5"
//	string                           -> `string "hi"` (Go-quoted)
//	[]byte                           -> "[]byte len=3"
//	[]any                            -> "[" + elements described recursively, joined by ", " + "]"
//	map[string]any                   -> "map[" + "key: description" pairs sorted by key, joined by ", " + "]"
//	anything else                    -> "unknown <type>" where <type> is %T, e.g. "unknown []int"
//
// error is checked before fmt.Stringer, so a type implementing both is
// described as an error.
//
// Use a single type switch (switch v := v.(type) { ... }).
func Describe(v any) string {
	return "" // TODO
}

// ValidationError reports an out-of-range value.
type ValidationError struct {
	Value  int
	Reason string
}

func (e *ValidationError) Error() string {
	return "invalid value " + strconv.Itoa(e.Value) + ": " + e.Reason
}

// Validate returns nil if 0 <= n <= 100. Otherwise it returns a
// *ValidationError whose Reason is "negative" or "greater than 100".
//
// TODO: this implementation looks right, and the error cases even pass,
// yet `Validate(50) != nil` is true. Explain why in a comment, then fix it.
func Validate(n int) error {
	var verr *ValidationError
	if n < 0 {
		verr = &ValidationError{Value: n, Reason: "negative"}
	} else if n > 100 {
		verr = &ValidationError{Value: n, Reason: "greater than 100"}
	}
	return verr
}
