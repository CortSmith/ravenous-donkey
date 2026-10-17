package csvjson

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// ConvertFast behaves like Convert, except that each object's keys appear in
// header order rather than sorted order. It avoids reflection and per-row
// allocations:
//
//   - csv.Reader.ReuseRecord reuses the record slice between rows;
//   - each key is JSON-encoded once, up front;
//   - values are escaped by hand into one reusable byte slice;
//   - output goes through a bufio.Writer (one write syscall per 4 KiB, not
//     per row), which must be flushed, and whose error must be checked.
func ConvertFast(dst io.Writer, src io.Reader) error {
	cr := csv.NewReader(src)
	cr.ReuseRecord = true // rec is only valid until the next Read

	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return ErrNoHeader
	}
	if err != nil {
		return fmt.Errorf("csvjson: reading header: %w", err)
	}
	// header is reused by the next Read, so copy it before keeping it.
	header = append([]string(nil), header...)
	if err := checkHeader(header); err != nil {
		return err
	}

	// Pre-encode `"key":` (with the leading ',' or '{') for every column.
	prefixes := make([][]byte, len(header))
	for i, key := range header {
		k, err := json.Marshal(key)
		if err != nil {
			return err
		}
		sep := byte(',')
		if i == 0 {
			sep = '{'
		}
		prefixes[i] = append(append([]byte{sep}, k...), ':')
	}

	bw := bufio.NewWriterSize(dst, 32*1024)
	line := make([]byte, 0, 1024)
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// Flush what we have so the partial output matches Convert's.
			return errors.Join(fmt.Errorf("csvjson: %w", err), bw.Flush())
		}
		line = line[:0]
		for i, v := range rec {
			line = append(line, prefixes[i]...)
			line = appendJSONString(line, v)
		}
		line = append(line, '}', '\n')
		if _, err := bw.Write(line); err != nil {
			return fmt.Errorf("csvjson: writing row: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("csvjson: writing row: %w", err)
	}
	return nil
}

const hexDigits = "0123456789abcdef"

// appendJSONString appends s to b as a JSON string literal, escaping the same
// characters encoding/json does by default (including <, > and & for safe
// embedding in HTML, and U+2028/U+2029). Invalid UTF-8 becomes U+FFFD.
func appendJSONString(b []byte, s string) []byte {
	b = append(b, '"')
	start := 0 // s[start:i] is pending, needs no escaping
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
				i++
				continue
			}
			b = append(b, s[start:i]...)
			switch c {
			case '"', '\\':
				b = append(b, '\\', c)
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			case '\b':
				b = append(b, '\\', 'b')
			case '\f':
				b = append(b, '\\', 'f')
			default:
				b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b = append(b, s[start:i]...)
			b = append(b, `\ufffd`...)
			i += size
			start = i
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			b = append(b, s[start:i]...)
			b = append(b, '\\', 'u', '2', '0', '2', hexDigits[r&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	b = append(b, s[start:]...)
	return append(b, '"')
}
