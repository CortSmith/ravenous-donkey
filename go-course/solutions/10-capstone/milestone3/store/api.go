package store

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Server is an HTTP API for DB.
type Server struct {
	db *DB
	lg *slog.Logger
}

// NewServer returns an HTTP server for db, logging to lg.
func NewServer(db *DB, lg *slog.Logger) *Server {
	return &Server{db: db, lg: lg}
}

// Register registers the server's routes on mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /kv/{key}", s.get)
	mux.HandleFunc("PUT /kv/{key}", s.put)
	mux.HandleFunc("DELETE /kv/{key}", s.delete)
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}

// get handles GET /kv/{key}.
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		s.writeJSON(w, http.StatusBadRequest, errorResp{Error: "key is required"})
		s.lg.InfoContext(r.Context(), "get request", "key", key, "status", http.StatusBadRequest)
		return
	}

	v, err := s.db.Get(key)
	if errors.Is(err, ErrNotFound) {
		s.writeJSON(w, http.StatusNotFound, errorResp{Error: "key not found"})
		s.lg.InfoContext(r.Context(), "get request", "key", key, "status", http.StatusNotFound)
		return
	}
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, errorResp{Error: "database error"})
		s.lg.ErrorContext(r.Context(), "get error", "key", key, "error", err)
		return
	}

	type getResp struct {
		Value string `json:"value"`
	}
	s.writeJSON(w, http.StatusOK, getResp{Value: v})
	s.lg.InfoContext(r.Context(), "get request", "key", key, "status", http.StatusOK)
}

// put handles PUT /kv/{key}.
func (s *Server) put(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		s.writeJSON(w, http.StatusBadRequest, errorResp{Error: "key is required"})
		s.lg.InfoContext(r.Context(), "put request", "key", key, "status", http.StatusBadRequest)
		return
	}

	var req putRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, errorResp{Error: "invalid request body"})
		s.lg.InfoContext(r.Context(), "put request", "key", key, "status", http.StatusBadRequest, "error", err)
		return
	}

	ttl := time.Duration(0)
	if req.TTL > 0 {
		ttl = time.Duration(req.TTL) * time.Millisecond
	}
	if err := s.db.Set(key, req.Value, ttl); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, errorResp{Error: "database error"})
		s.lg.ErrorContext(r.Context(), "put error", "key", key, "error", err)
		return
	}

	type putResp struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	s.writeJSON(w, http.StatusOK, putResp{Key: key, Value: req.Value})
	s.lg.InfoContext(r.Context(), "put request", "key", key, "status", http.StatusOK)
}

// delete handles DELETE /kv/{key}.
func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		s.writeJSON(w, http.StatusBadRequest, errorResp{Error: "key is required"})
		s.lg.InfoContext(r.Context(), "delete request", "key", key, "status", http.StatusBadRequest)
		return
	}

	ok, err := s.db.Delete(key)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, errorResp{Error: "database error"})
		s.lg.ErrorContext(r.Context(), "delete error", "key", key, "error", err)
		return
	}

	if !ok {
		s.writeJSON(w, http.StatusNotFound, errorResp{Error: "key not found"})
		s.lg.InfoContext(r.Context(), "delete request", "key", key, "status", http.StatusNotFound)
		return
	}

	type delResp struct {
		Key string `json:"key"`
	}
	s.writeJSON(w, http.StatusOK, delResp{Key: key})
	s.lg.InfoContext(r.Context(), "delete request", "key", key, "status", http.StatusOK)
}
