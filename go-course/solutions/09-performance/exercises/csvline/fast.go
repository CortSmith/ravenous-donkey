package csvline

// AppendLine is the allocation-free variant of Line: it appends the encoded
// record to dst and returns the extended slice, so a caller that reuses dst
// (dst = AppendLine(dst[:0], fields)) pays zero allocations per record once
// dst has grown large enough. It scans each field once, byte by byte, instead
// of calling ContainsAny, Count and IndexByte separately.
func AppendLine(dst []byte, fields []string) []byte {
	for i, f := range fields {
		if i > 0 {
			dst = append(dst, ',')
		}
		quote := false
		for j := 0; j < len(f); j++ {
			if c := f[j]; c == ',' || c == '"' || c == '\r' || c == '\n' {
				quote = true
				break
			}
		}
		if !quote {
			dst = append(dst, f...)
			continue
		}
		dst = append(dst, '"')
		start := 0
		for j := 0; j < len(f); j++ {
			if f[j] == '"' {
				dst = append(dst, f[start:j+1]...)
				dst = append(dst, '"')
				start = j + 1
			}
		}
		dst = append(dst, f[start:]...)
		dst = append(dst, '"')
	}
	return dst
}
