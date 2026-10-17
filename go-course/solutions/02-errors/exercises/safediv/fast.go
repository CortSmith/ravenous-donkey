package safediv

import (
	"fmt"
	"runtime"
	"strings"
)

// LazyPanicError is like PanicError but records only program counters at
// recovery time. debug.Stack() formats the whole goroutine stack into text
// (symbol lookup, file/line resolution, a growing buffer) on EVERY panic,
// even if nobody ever reads the trace. runtime.Callers just copies return
// addresses; the expensive symbolization happens in Stack(), on demand.
type LazyPanicError struct {
	Value any
	pcs   []uintptr
}

func (e *LazyPanicError) Error() string { return fmt.Sprintf("panic: %v", e.Value) }

// Unwrap exposes error panic values, exactly like PanicError.Unwrap.
func (e *LazyPanicError) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}

// Stack symbolizes the recorded program counters into "function\n\tfile:line"
// lines. It is computed on each call; cache it if you call it repeatedly.
func (e *LazyPanicError) Stack() string {
	var b strings.Builder
	frames := runtime.CallersFrames(e.pcs)
	for {
		f, more := frames.Next()
		fmt.Fprintf(&b, "%s\n\t%s:%d\n", f.Function, f.File, f.Line)
		if !more {
			break
		}
	}
	return b.String()
}

// SafelyFast is Safely with a cheaper panic path (see LazyPanicError).
// The non-panic path is identical: a deferred closure and one recover call.
func SafelyFast(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			pcs := make([]uintptr, 32)
			// Skip runtime.Callers itself and this deferred closure.
			n := runtime.Callers(2, pcs)
			err = &LazyPanicError{Value: r, pcs: pcs[:n]}
		}
	}()
	return f()
}
