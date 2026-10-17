package describe

import (
	"fmt"
	"slices"
	"strconv"
)

// DescribeFast returns the same result as Describe but builds it in a single
// growing []byte instead of creating one string per element and joining.
func DescribeFast(v any) string {
	return string(AppendDescribe(make([]byte, 0, 64), v))
}

// AppendDescribe appends the description of v to dst and returns the
// extended buffer, following the strconv.AppendX convention. Callers that
// describe many values can reuse dst and pay zero allocations for scalar
// values.
//
// The integer cases are spelled out one by one: that is more code than
// Describe's single multi-type case, but each case gets a typed v, so no
// fmt.Sprintf (and its reflection and boxing) is needed.
func AppendDescribe(dst []byte, v any) []byte {
	switch v := v.(type) {
	case nil:
		return append(dst, "nil"...)
	case error:
		return append(append(dst, "error: "...), v.Error()...)
	case fmt.Stringer:
		return append(append(dst, "Stringer: "...), v.String()...)
	case bool:
		return strconv.AppendBool(append(dst, "bool "...), v)
	case int:
		return strconv.AppendInt(append(dst, "int "...), int64(v), 10)
	case int8:
		return strconv.AppendInt(append(dst, "int8 "...), int64(v), 10)
	case int16:
		return strconv.AppendInt(append(dst, "int16 "...), int64(v), 10)
	case int32:
		return strconv.AppendInt(append(dst, "int32 "...), int64(v), 10)
	case int64:
		return strconv.AppendInt(append(dst, "int64 "...), v, 10)
	case uint:
		return strconv.AppendUint(append(dst, "uint "...), uint64(v), 10)
	case uint8:
		return strconv.AppendUint(append(dst, "uint8 "...), uint64(v), 10)
	case uint16:
		return strconv.AppendUint(append(dst, "uint16 "...), uint64(v), 10)
	case uint32:
		return strconv.AppendUint(append(dst, "uint32 "...), uint64(v), 10)
	case uint64:
		return strconv.AppendUint(append(dst, "uint64 "...), v, 10)
	case float32:
		return strconv.AppendFloat(append(dst, "float32 "...), float64(v), 'g', -1, 32)
	case float64:
		return strconv.AppendFloat(append(dst, "float64 "...), v, 'g', -1, 64)
	case string:
		return strconv.AppendQuote(append(dst, "string "...), v)
	case []byte:
		return strconv.AppendInt(append(dst, "[]byte len="...), int64(len(v)), 10)
	case []any:
		dst = append(dst, '[')
		for i, elem := range v {
			if i > 0 {
				dst = append(dst, ", "...)
			}
			dst = AppendDescribe(dst, elem)
		}
		return append(dst, ']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		dst = append(dst, "map["...)
		for i, k := range keys {
			if i > 0 {
				dst = append(dst, ", "...)
			}
			dst = append(append(dst, k...), ": "...)
			dst = AppendDescribe(dst, v[k])
		}
		return append(dst, ']')
	default:
		return fmt.Appendf(dst, "unknown %T", v)
	}
}
