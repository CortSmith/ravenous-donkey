// Package routes exposes a notes Store over HTTP using the method and
// wildcard patterns of net/http's ServeMux (Go 1.22+).
package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	h := &handlers{store: s}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.index)
	mux.HandleFunc("GET /notes", h.list)
	mux.HandleFunc("POST /notes", h.create)
	mux.HandleFunc("GET /notes/latest", h.latest) // more specific than /notes/{id}, so it wins
	mux.HandleFunc("GET /notes/{id}", h.get)
	mux.HandleFunc("DELETE /notes/{id}", h.delete)
	mux.HandleFunc("GET /files/{path...}", h.file)
	return mux
}

// handlers holds the dependencies shared by every handler.
type handlers struct {
	store *Store
}

func (h *handlers) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "notes API")
}

func (h *handlers) list(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.List())
}

func (h *handlers) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}
	n := h.store.Add(text)
	w.Header().Set("Location", "/notes/"+strconv.Itoa(n.ID))
	writeJSON(w, http.StatusCreated, n)
}

func (h *handlers) latest(w http.ResponseWriter, r *http.Request) {
	n, ok := h.store.Latest()
	if !ok {
		writeError(w, http.StatusNotFound, "no notes yet")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *handlers) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	n, found := h.store.Get(id)
	if !found {
		writeError(w, http.StatusNotFound, "note not found")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *handlers) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if !h.store.Delete(id) {
		writeError(w, http.StatusNotFound, "note not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) file(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "file: %s\n", r.PathValue("path"))
}

// parseID reads the {id} wildcard. On failure it writes a 400 and returns
// false, so callers just return.
func parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // values here always encode; write errors mean the client left
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
