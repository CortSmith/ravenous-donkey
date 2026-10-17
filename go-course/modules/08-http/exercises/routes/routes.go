// Package routes exposes a notes Store over HTTP using the method and
// wildcard patterns of net/http's ServeMux (Go 1.22+).
package routes

import (
	"net/http"
)

// Routes returns the HTTP API for s:
//
//	GET    /                  -> 200 text/plain "notes API\n" (exactly "/", nothing else)
//	GET    /notes             -> 200 JSON array of notes ordered by ID ([] when empty)
//	POST   /notes             -> 201 JSON note, Location: /notes/{id}
//	GET    /notes/latest      -> 200 JSON note with the highest ID, 404 if none
//	GET    /notes/{id}        -> 200 JSON note, 400 bad id, 404 unknown id
//	DELETE /notes/{id}        -> 204 no body, 400 bad id, 404 unknown id
//	GET    /files/{path...}   -> 200 text/plain "file: <path>\n"
//
// Errors are JSON objects {"error": "..."}. Methods not listed for a path get
// ServeMux's automatic 405 with an Allow header; unknown paths get 404.
func Routes(s *Store) http.Handler {
	// TODO: implement Routes
	h := &handlers{store: s}
	mux := http.NewServeMux()
	// TODO: register handlers on mux
	_ = h
	return mux
}

// handlers holds the dependencies shared by every handler.
type handlers struct {
	store *Store
}

func (h *handlers) index(w http.ResponseWriter, r *http.Request) {
	// TODO: implement index handler
	_ = w
	_ = r
}

func (h *handlers) list(w http.ResponseWriter, r *http.Request) {
	// TODO: implement list handler
	_ = w
	_ = r
}

func (h *handlers) create(w http.ResponseWriter, r *http.Request) {
	// TODO: implement create handler
	_ = w
	_ = r
}

func (h *handlers) latest(w http.ResponseWriter, r *http.Request) {
	// TODO: implement latest handler
	_ = w
	_ = r
}

func (h *handlers) get(w http.ResponseWriter, r *http.Request) {
	// TODO: implement get handler
	_ = w
	_ = r
}

func (h *handlers) delete(w http.ResponseWriter, r *http.Request) {
	// TODO: implement delete handler
	_ = w
	_ = r
}

func (h *handlers) file(w http.ResponseWriter, r *http.Request) {
	// TODO: implement file handler
	_ = w
	_ = r
}

// parseID reads the {id} wildcard. On failure it writes a 400 and returns
// false, so callers just return.
func parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	// TODO: implement parseID
	_ = w
	_ = r
	return 0, false
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	// TODO: implement writeJSON
	_ = w
	_ = status
	_ = v
}

func writeError(w http.ResponseWriter, status int, msg string) {
	// TODO: implement writeError
	_ = w
	_ = status
	_ = msg
}
