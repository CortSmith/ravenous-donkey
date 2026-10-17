package jsonio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// bufPool recycles encode buffers between responses. Buffers that grew past
// maxPooledBuf are dropped so that one huge response does not pin memory
// forever (see the sync.Pool caveats in the lesson).
var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

const maxPooledBuf = 64 << 10

// WriteJSONFast behaves like WriteJSON but reuses encode buffers from a
// sync.Pool and sets the header map entry directly (the key is already in
// canonical form), trading a little clarity for fewer allocations.
func WriteJSONFast(w http.ResponseWriter, status int, v any) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		if buf.Cap() <= maxPooledBuf {
			bufPool.Put(buf)
		}
	}()

	if err := json.NewEncoder(buf).Encode(v); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return fmt.Errorf("encode response: %w", err)
	}
	w.Header()["Content-Type"] = jsonContentType
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}

// jsonContentType is shared by every response; net/http never mutates it.
var jsonContentType = []string{"application/json"}
