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
	"path"
	"strings"
	"time"
	"unicode/utf8"
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

// maxURLLength bounds a link's URL, in characters (decision 11).
const maxURLLength = 2048

// maxNameLength bounds a chosen name, in characters.
const maxNameLength = 64

// Server serves shrt's HTTP API, redirect and health check.
type Server struct {
	store   *Store
	baseURL string
	// selfHost and selfPath are baseURL's host and path, which a link's
	// URL may not point back into.
	selfHost, selfPath string
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
	// ConfigFromEnv checked baseURL, so an error leaves only links into
	// shrt itself unchecked.
	self, _ := url.Parse(baseURL)
	if self == nil {
		self = &url.URL{}
	}
	s := &Server{
		store:     store,
		baseURL:   baseURL,
		selfHost:  normalHost(self.Hostname()),
		selfPath:  strings.TrimRight(self.Path, "/"),
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
		URL  string  `json:"url"`
		Name *string `json:"name"`
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
	if err := s.checkURL(req.URL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	link := Link{URL: req.URL, CreatedAt: s.now().UTC().Truncate(time.Second)}
	if req.Name != nil {
		s.createNamed(w, r, link, *req.Name)
		return
	}
	for range maxDraws {
		existing, err := s.store.GeneratedFor(r.Context(), link.URL)
		if err == nil {
			writeJSON(w, http.StatusOK, s.present(existing))
			return
		}
		if !errors.Is(err, ErrNotFound) {
			s.internalError(w, r, err)
			return
		}
		link.Code = s.newCode()
		inserted, err := s.store.InsertGenerated(r.Context(), link)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		if inserted {
			writeJSON(w, http.StatusCreated, s.present(link))
			return
		}
		// The code was taken, or another call just gave the URL a code,
		// which the next round finds.
	}
	s.internalError(w, r, fmt.Errorf("no free code in %d draws", maxDraws))
}

// createNamed stores link under the chosen name, unless the name is
// invalid or taken (ADR 0004). A name always makes its own link, even for
// a URL that already has one (decision 10).
func (s *Server) createNamed(w http.ResponseWriter, r *http.Request, link Link, name string) {
	if !validName(name) {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("name must be 1 to %d characters of a-z, 0-9 and -", maxNameLength))
		return
	}
	link.Code = name
	inserted, err := s.store.InsertNamed(r.Context(), link)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if !inserted {
		writeError(w, http.StatusConflict, fmt.Sprintf("name %q is taken", name))
		return
	}
	writeJSON(w, http.StatusCreated, s.present(link))
}

func validName(name string) bool {
	if name == "" || len(name) > maxNameLength {
		return false
	}
	for _, c := range []byte(name) {
		if !('a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// checkURL accepts only absolute http:// and https:// URLs of at most
// maxURLLength characters that don't point back into shrt's own /x/
// (decision 11).
func (s *Server) checkURL(raw string) error {
	if raw == "" {
		return errors.New("url is required")
	}
	if utf8.RuneCountInString(raw) > maxURLLength {
		return fmt.Errorf("url is longer than %d characters", maxURLLength)
	}
	u, err := url.Parse(raw)
	if err != nil || !isAbsoluteHTTPURL(u) {
		return errors.New("url must be an absolute http:// or https:// URL")
	}
	if s.isShortLink(u) {
		return errors.New("url points to a short link of shrt itself")
	}
	return nil
}

// isShortLink reports whether u leads to shrt's /x/, whatever its scheme
// and port, and with its path cleaned as nginx and browsers clean it.
func (s *Server) isShortLink(u *url.URL) bool {
	if normalHost(u.Hostname()) != s.selfHost {
		return false
	}
	p := path.Clean("/" + u.Path)
	x := s.selfPath + "/x"
	return p == x || strings.HasPrefix(p, x+"/")
}

// normalHost lowercases host and drops the dot that ends a fully qualified
// name.
func normalHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
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
