// Package describe practises type switches, type assertions and the
// "nil interface vs nil pointer" gotcha.
package describe

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

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
func Describe(v any) string {
	switch v := v.(type) {
	case nil:
		return "nil"
	case error:
		return "error: " + v.Error()
	case fmt.Stringer:
		return "Stringer: " + v.String()
	case bool:
		return "bool " + strconv.FormatBool(v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		// In a multi-type case v keeps the type any, so let fmt do the work.
		return fmt.Sprintf("%T %d", v, v)
	case float32:
		return "float32 " + strconv.FormatFloat(float64(v), 'g', -1, 32)
	case float64:
		return "float64 " + strconv.FormatFloat(v, 'g', -1, 64)
	case string:
		return "string " + strconv.Quote(v)
	case []byte:
		return "[]byte len=" + strconv.Itoa(len(v))
	case []any:
		parts := make([]string, len(v))
		for i, elem := range v {
			parts[i] = Describe(elem)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + ": " + Describe(v[k])
		}
		return "map[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("unknown %T", v)
	}
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
// The fix: return the untyped nil literal on the success path. Returning a
// nil *ValidationError through the error interface would produce a non-nil
// interface value (type=*ValidationError, value=nil).
func Validate(n int) error {
	switch {
	case n < 0:
		return &ValidationError{Value: n, Reason: "negative"}
	case n > 100:
		return &ValidationError{Value: n, Reason: "greater than 100"}
	}
	return nil
}
