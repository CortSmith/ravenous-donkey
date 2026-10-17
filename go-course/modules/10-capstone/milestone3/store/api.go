package store

import (
	"log/slog"
	"net/http"
)

// Server is an HTTP API for DB.
type Server struct {
	db *DB
	lg *slog.Logger
}

// NewServer returns an HTTP server for db, logging to lg.
func NewServer(db *DB, lg *slog.Logger) *Server {
	// TODO
	return nil
}

// Register registers the server's routes on mux.
func (s *Server) Register(mux *http.ServeMux) {
	// TODO: wire up handlers for:
	//   GET  /kv/{key}    (read)
	//   PUT  /kv/{key}    (write with optional TTL in request body)
	//   DELETE /kv/{key}  (delete)
}

// putRequest is the request body for PUT /kv/{key}.
type putRequest struct {
	Value string `json:"value"`
	TTL   int64  `json:"ttl,omitempty"` // milliseconds
}

// errorResp is the response body on error.
type errorResp struct {
	Error string `json:"error"`
}

// writeJSON writes v as JSON with the given status to w.
func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	// TODO
}

// get handles GET /kv/{key}.
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	// TODO
}

// put handles PUT /kv/{key}.
func (s *Server) put(w http.ResponseWriter, r *http.Request) {
	// TODO
}

// delete handles DELETE /kv/{key}.
func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	// TODO
}
