package shortener

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Limits enforced by the API.
const (
	MaxBodyBytes = 4 << 10 // largest accepted POST /links body
	MaxURLLen    = 2048    // longest accepted target URL
	MinCodeLen   = 4       // shortest accepted custom code
	MaxCodeLen   = 32      // longest accepted custom code
)

const (
	maxCodeTries  = 5 // attempts to find an unused generated code
	defaultBase   = "http://localhost:8080"
	errInternal   = "internal server error"
	errBadJSONMsg = `request body must be a JSON object like {"url": "https://..."}`
)

// Config configures a Server. Zero fields get sensible defaults.
type Config struct {
	Store   Store                  // default: NewMemoryStore()
	Logger  *slog.Logger           // default: discard all logs
	BaseURL string                 // prefix for short_url; default "http://localhost:8080"
	NewCode func() (string, error) // default: RandomCode
	Now     func() time.Time       // default: time.Now
}

// Server is the URL-shortener HTTP API. Create it with NewServer.
type Server struct {
	store   Store
	logger  *slog.Logger
	baseURL string
	newCode func() (string, error)
	now     func() time.Time
	handler http.Handler
}

// NewServer returns a Server with routes:
//
//	POST /links               create a link   -> 201
//	GET  /{code}              redirect        -> 302
//	GET  /links/{code}/stats  link statistics -> 200
func NewServer(cfg Config) *Server {
	s := &Server{
		store:   cfg.Store,
		logger:  cfg.Logger,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		newCode: cfg.NewCode,
		now:     cfg.Now,
	}
	if s.store == nil {
		s.store = NewMemoryStore()
	}
	if s.logger == nil {
		s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if s.baseURL == "" {
		s.baseURL = defaultBase
	}
	if s.newCode == nil {
		s.newCode = RandomCode
	}
	if s.now == nil {
		s.now = time.Now
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /links", s.handleCreate)
	mux.HandleFunc("GET /links/{code}/stats", s.handleStats)
	mux.HandleFunc("GET /{code}", s.handleRedirect)
	s.handler = s.logRequests(mux)
	return s
}

// ServeHTTP makes Server an http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// linkResponse is the JSON shape returned for a link.
type linkResponse struct {
	Code      string    `json:"code"`
	URL       string    `json:"url"`
	ShortURL  string    `json:"short_url"`
	Visits    int64     `json:"visits"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) toResponse(l Link) linkResponse {
	return linkResponse{
		Code:      l.Code,
		URL:       l.URL,
		ShortURL:  s.baseURL + "/" + l.Code,
		Visits:    l.Visits,
		CreatedAt: l.CreatedAt,
	}
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL  string `json:"url"`
		Code string `json:"code"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, errBadJSONMsg)
		return
	}
	if msg := validateURL(in.URL); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	link := Link{URL: in.URL, CreatedAt: s.now().UTC()}
	if in.Code != "" {
		if !validCode(in.Code) {
			writeError(w, http.StatusBadRequest, "code must be 4-32 characters of A-Z, a-z, 0-9, '-' or '_'")
			return
		}
		link.Code = in.Code
		if err := s.store.Create(r.Context(), link); err != nil {
			if errors.Is(err, ErrExists) {
				writeError(w, http.StatusConflict, "code already in use")
				return
			}
			s.internalError(w, r, "create link", err)
			return
		}
	} else if err := s.createWithGeneratedCode(r, &link); err != nil {
		s.internalError(w, r, "create link", err)
		return
	}

	resp := s.toResponse(link)
	w.Header().Set("Location", resp.ShortURL)
	writeJSON(w, http.StatusCreated, resp)
}

// createWithGeneratedCode stores link under a fresh random code, retrying a
// few times on collision.
func (s *Server) createWithGeneratedCode(r *http.Request, link *Link) error {
	for range maxCodeTries {
		code, err := s.newCode()
		if err != nil {
			return err
		}
		link.Code = code
		err = s.store.Create(r.Context(), *link)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrExists) {
			return err
		}
	}
	return errors.New("could not find an unused code")
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	link, err := s.store.Visit(r.Context(), r.PathValue("code"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "link not found")
			return
		}
		s.internalError(w, r, "visit link", err)
		return
	}
	http.Redirect(w, r, link.URL, http.StatusFound)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	link, err := s.store.Get(r.Context(), r.PathValue("code"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "link not found")
			return
		}
		s.internalError(w, r, "get link", err)
		return
	}
	writeJSON(w, http.StatusOK, s.toResponse(link))
}

// internalError logs the real error and sends the client a generic message:
// internal details (SQL, file paths) must not leak into responses.
func (s *Server) internalError(w http.ResponseWriter, r *http.Request, op string, err error) {
	s.logger.ErrorContext(r.Context(), op+" failed", "err", err, "path", r.URL.Path)
	writeError(w, http.StatusInternalServerError, errInternal)
}

// validateURL returns a client-facing message, or "" when raw is acceptable.
func validateURL(raw string) string {
	if raw == "" {
		return "url is required"
	}
	if len(raw) > MaxURLLen {
		return "url is too long"
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "url must be an absolute http or https URL"
	}
	return ""
}

// validCode reports whether code is 4-32 characters of [A-Za-z0-9_-].
func validCode(code string) bool {
	if len(code) < MinCodeLen || len(code) > MaxCodeLen {
		return false
	}
	for i := 0; i < len(code); i++ {
		c := code[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // our response types always encode
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// logRequests logs one Info record per request.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.logger.InfoContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration", time.Since(start),
		)
	})
}

// statusWriter captures the response status for logging.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status, w.wroteHeader = code, true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(p)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
