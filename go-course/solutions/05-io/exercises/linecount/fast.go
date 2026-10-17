package linecount

import (
	"bytes"
	"errors"
	"io"
)

// CountFast is a performance-tuned Count. Instead of asking bufio for one
// line at a time, it reads large chunks and uses bytes.IndexByte (which is
// implemented in assembly with SIMD on common platforms) to jump from one
// '\n' to the next. The only allocation is the 64 KiB read buffer.
func CountFast(r io.Reader) (Stats, error) {
	var (
		s       Stats
		buf     = make([]byte, 64*1024)
		lineLen int  // bytes of the current, unfinished line
		lastCR  bool // the last content byte of the current line is '\r'
	)
	for {
		n, err := r.Read(buf)
		b := buf[:n]
		for len(b) > 0 {
			i := bytes.IndexByte(b, '\n')
			if i < 0 {
				lineLen += len(b)
				lastCR = b[len(b)-1] == '\r'
				break
			}
			lineLen += i
			if i > 0 {
				lastCR = b[i-1] == '\r'
			}
			if lastCR {
				lineLen-- // "\r\n" terminator: the '\r' is not content
			}
			s.Lines++
			s.Longest = max(s.Longest, lineLen)
			lineLen, lastCR = 0, false
			b = b[i+1:]
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return s, err
			}
			if lineLen > 0 { // unterminated last line; a trailing '\r' is content
				s.Lines++
				s.Longest = max(s.Longest, lineLen)
			}
			return s, nil
		}
	}
}
