package shrt

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Link is a short link, as the API answers it.
type Link struct {
	Code      string    `json:"code"`
	ShortURL  string    `json:"short_url"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

// maxDraws bounds how often createLink draws a code when they collide. At
// 62^6 codes, even a million links make a single collision a 1 in 57,000
// chance.
const maxDraws = 5

// maxBodyBytes bounds an API request body.
const maxBodyBytes = 64 << 10

// Server serves shrt's HTTP API, redirect and health check.
type Server struct {
	store   *Store
	baseURL string
	// tokenHash is the SHA-256 of the API token, so that comparing it
	// takes the same time whatever the length of the token presented.
	tokenHash [sha256.Size]byte
	hasToken  bool
	mux       *http.ServeMux

	newCode func() string
	now     func() time.Time
}

// NewServer serves the links in store. baseURL, without a trailing slash,
// prefixes /x/<code> in a link's short_url. With an empty token, every API
// call is refused.
func NewServer(store *Store, baseURL, token string) *Server {
	s := &Server{
		store:     store,
		baseURL:   baseURL,
		tokenHash: sha256.Sum256([]byte(token)),
		hasToken:  token != "",
		mux:       http.NewServeMux(),
		newCode:   newCode,
		now:       time.Now,
	}

	api := http.NewServeMux()
	api.HandleFunc("POST /api/links", s.createLink)
	api.HandleFunc("GET /api/links", s.listLinks)
	api.HandleFunc("GET /api/links/{code}", s.showLink)
	api.HandleFunc("DELETE /api/links/{code}", s.deleteLink)
	// Answers what no other pattern matches, wrong methods included, so
	// that API errors stay JSON.
	api.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such API call")
	})
	s.mux.Handle("/api/", s.requireToken(api))

	s.mux.HandleFunc("GET /x/{code}", s.redirect)
	s.mux.HandleFunc("GET /healthz", s.healthz)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if msg := s.refusal(r); msg != "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, msg)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// refusal says why r may not call the API, or is empty when it may.
func (s *Server) refusal(r *http.Request) string {
	if !s.hasToken {
		return "no API token is configured"
	}
	scheme, token, _ := strings.Cut(r.Header.Get("Authorization"), " ")
	got := sha256.Sum256([]byte(token))
	if !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare(got[:], s.tokenHash[:]) != 1 {
		return "missing or wrong bearer token"
	}
	return ""
}

func (s *Server) createLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("body is larger than %d bytes", maxBodyBytes))
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid JSON body: more than one value")
		return
	}
	if err := checkURL(req.URL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	link := Link{URL: req.URL, CreatedAt: s.now().UTC().Truncate(time.Second)}
	for range maxDraws {
		link.Code = s.newCode()
		inserted, err := s.store.Insert(r.Context(), link)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		if inserted {
			writeJSON(w, http.StatusCreated, s.present(link))
			return
		}
	}
	s.internalError(w, r, fmt.Errorf("no free code in %d draws", maxDraws))
}

// checkURL accepts only absolute http:// and https:// URLs.
func checkURL(raw string) error {
	if raw == "" {
		return errors.New("url is required")
	}
	u, err := url.Parse(raw)
	if err != nil || !isAbsoluteHTTPURL(u) {
		return errors.New("url must be an absolute http:// or https:// URL")
	}
	return nil
}

func isAbsoluteHTTPURL(u *url.URL) bool {
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (s *Server) listLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.store.List(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	for i := range links {
		links[i] = s.present(links[i])
	}
	writeJSON(w, http.StatusOK, links)
}

func (s *Server) showLink(w http.ResponseWriter, r *http.Request) {
	link, err := s.store.Get(r.Context(), r.PathValue("code"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such link")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.present(link))
}

func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request) {
	err := s.store.Delete(r.Context(), r.PathValue("code"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such link")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// present fills in the short_url that the store doesn't keep.
func (s *Server) present(link Link) Link {
	link.ShortURL = s.baseURL + "/x/" + link.Code
	return link
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	link, err := s.store.Get(r.Context(), r.PathValue("code"))
	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "looking up link", "path", r.URL.Path, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Location", link.URL)
	w.WriteHeader(http.StatusFound)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		slog.ErrorContext(r.Context(), "health check", "err", err)
		http.Error(w, "database unreachable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, "ok\n")
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "API call failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
