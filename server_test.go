package shrt

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testToken   = "s3cret-token"
	testBaseURL = "https://hypr.sh"
)

var testNow = time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)

// newTestServer serves shrt over a fresh SQLite file in a temporary
// directory, with the clock fixed at testNow.
func newTestServer(t *testing.T, token string) (*httptest.Server, *Server) {
	t.Helper()
	return serveStore(t, openTestStore(t, filepath.Join(t.TempDir(), "shrt.db")), token)
}

func openTestStore(t *testing.T, path string) *Store {
	t.Helper()
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func serveStore(t *testing.T, store *Store, token string) (*httptest.Server, *Server) {
	t.Helper()
	srv := NewServer(store, testBaseURL, token)
	srv.now = func() time.Time { return testNow }
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts, srv
}

// noRedirects is a client that hands back redirects instead of following them.
var noRedirects = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func do(t *testing.T, method, url, auth, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := noRedirects.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func create(t *testing.T, ts *httptest.Server, url string) Link {
	t.Helper()
	resp := do(t, "POST", ts.URL+"/api/links", "Bearer "+testToken, `{"url": "`+url+`"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/links: status %d, want 201", resp.StatusCode)
	}
	var link Link
	decode(t, resp, &link)
	return link
}

func decode(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type %q, want application/json", ct)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
}

func wantError(t *testing.T, resp *http.Response, status int) {
	t.Helper()
	if resp.StatusCode != status {
		t.Errorf("status %d, want %d", resp.StatusCode, status)
	}
	var body map[string]string
	decode(t, resp, &body)
	if body["error"] == "" || len(body) != 1 {
		t.Errorf("body %v, want {\"error\": \"...\"}", body)
	}
}

func TestCreateLink(t *testing.T) {
	ts, _ := newTestServer(t, testToken)

	for _, url := range []string{"https://example.com/some/page?q=1", "http://example.com"} {
		resp := do(t, "POST", ts.URL+"/api/links", "Bearer "+testToken, `{"url": "`+url+`"}`)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("%s: status %d, want 201", url, resp.StatusCode)
		}
		var got map[string]string
		decode(t, resp, &got)
		if !codePattern.MatchString(got["code"]) {
			t.Errorf("code %q, want 6 characters of a-z A-Z 0-9", got["code"])
		}
		want := map[string]string{
			"code":       got["code"],
			"short_url":  "https://hypr.sh/x/" + got["code"],
			"url":        url,
			"created_at": "2026-10-07T08:00:00Z",
		}
		if len(got) != len(want) {
			t.Errorf("body %v, want %v", got, want)
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s: %q, want %q", k, got[k], v)
			}
		}
	}
}

func TestCreateLinkShortURLUsesBaseURL(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "shrt.db"))
	srv := NewServer(store, "http://localhost:8080", testToken)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	link := create(t, ts, "https://example.com")
	if want := "http://localhost:8080/x/" + link.Code; link.ShortURL != want {
		t.Errorf("short_url %q, want %q", link.ShortURL, want)
	}
}

func TestCreateLinkDrawsAgainOnCollision(t *testing.T) {
	ts, srv := newTestServer(t, testToken)
	codes := []string{"aaaaaa", "aaaaaa", "aaaaaa", "bbbbbb"}
	srv.newCode = func() string {
		code := codes[0]
		codes = codes[1:]
		return code
	}

	first := create(t, ts, "https://example.com/first")
	second := create(t, ts, "https://example.com/second")
	if first.Code != "aaaaaa" || second.Code != "bbbbbb" {
		t.Errorf("codes %q and %q, want aaaaaa and bbbbbb", first.Code, second.Code)
	}
	if len(codes) != 0 {
		t.Errorf("%d codes left undrawn", len(codes))
	}

	// The first link is untouched by the collision.
	resp := do(t, "GET", ts.URL+"/x/aaaaaa", "", "")
	if got := resp.Header.Get("Location"); got != "https://example.com/first" {
		t.Errorf("Location %q, want https://example.com/first", got)
	}
}

func TestCreateLinkGivesUpAfterRepeatedCollisions(t *testing.T) {
	ts, srv := newTestServer(t, testToken)
	srv.newCode = func() string { return "aaaaaa" }
	create(t, ts, "https://example.com/first")

	resp := do(t, "POST", ts.URL+"/api/links", "Bearer "+testToken, `{"url": "https://example.com/second"}`)
	wantError(t, resp, http.StatusInternalServerError)
}

func TestCreateLinkRejectsBadRequests(t *testing.T) {
	ts, _ := newTestServer(t, testToken)

	for name, body := range map[string]string{
		"not JSON":      `url=https://example.com`,
		"no url":        `{}`,
		"empty url":     `{"url": ""}`,
		"url not text":  `{"url": 42}`,
		"other scheme":  `{"url": "ftp://example.com/file"}`,
		"javascript":    `{"url": "javascript:alert(1)"}`,
		"relative":      `{"url": "/x/abc"}`,
		"no host":       `{"url": "https:///path"}`,
		"unknown field": `{"url": "https://example.com", "name": "ferien"}`,
		"trailing data": `{"url": "https://example.com"} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp := do(t, "POST", ts.URL+"/api/links", "Bearer "+testToken, body)
			wantError(t, resp, http.StatusBadRequest)
		})
	}
}

func TestCreateLinkRejectsLargeBody(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	body := `{"url": "https://example.com/` + strings.Repeat("a", maxBodyBytes) + `"}`
	resp := do(t, "POST", ts.URL+"/api/links", "Bearer "+testToken, body)
	wantError(t, resp, http.StatusRequestEntityTooLarge)
}

func TestAPIUnknownCallIsJSONNotFound(t *testing.T) {
	ts, _ := newTestServer(t, testToken)

	for _, call := range []string{"GET /api/nope", "PUT /api/links", "GET /api/"} {
		method, path, _ := strings.Cut(call, " ")
		resp := do(t, method, ts.URL+path, "Bearer "+testToken, "")
		wantError(t, resp, http.StatusNotFound)
	}
}

func TestAPIRefusesMissingOrWrongToken(t *testing.T) {
	ts, _ := newTestServer(t, testToken)

	for name, auth := range map[string]string{
		"missing":      "",
		"wrong":        "Bearer not-the-token",
		"prefix":       "Bearer " + testToken[:4],
		"longer":       "Bearer " + testToken + "x",
		"no scheme":    testToken,
		"other scheme": "Basic " + testToken,
		"empty bearer": "Bearer ",
	} {
		t.Run(name, func(t *testing.T) {
			resp := do(t, "POST", ts.URL+"/api/links", auth, `{"url": "https://example.com"}`)
			wantError(t, resp, http.StatusUnauthorized)
			if got := resp.Header.Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("WWW-Authenticate %q, want Bearer", got)
			}
		})
	}
}

func TestAPIAcceptsCaseInsensitiveBearerScheme(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	resp := do(t, "POST", ts.URL+"/api/links", "bearer "+testToken, `{"url": "https://example.com"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status %d, want 201", resp.StatusCode)
	}
}

func TestAPIRefusesEveryCallWithoutConfiguredToken(t *testing.T) {
	ts, _ := newTestServer(t, "")

	for _, auth := range []string{"", "Bearer ", "Bearer anything"} {
		resp := do(t, "POST", ts.URL+"/api/links", auth, `{"url": "https://example.com"}`)
		wantError(t, resp, http.StatusUnauthorized)
	}
	// Paths the API doesn't serve are refused too, before routing.
	resp := do(t, "GET", ts.URL+"/api/anything", "Bearer anything", "")
	wantError(t, resp, http.StatusUnauthorized)
}

func TestRedirect(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	link := create(t, ts, "https://example.com/some/page?q=1&r=2#frag")

	for _, method := range []string{"GET", "HEAD"} {
		resp := do(t, method, ts.URL+"/x/"+link.Code, "", "")
		if resp.StatusCode != http.StatusFound {
			t.Errorf("%s: status %d, want 302", method, resp.StatusCode)
		}
		if got := resp.Header.Get("Location"); got != link.URL {
			t.Errorf("%s: Location %q, want %q", method, got, link.URL)
		}
	}
}

func TestRedirectIsCaseSensitive(t *testing.T) {
	ts, srv := newTestServer(t, testToken)
	srv.newCode = func() string { return "AbCdEf" }
	create(t, ts, "https://example.com")

	resp := do(t, "GET", ts.URL+"/x/abcdef", "", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status %d, want 404", resp.StatusCode)
	}
}

func TestRedirectUnknownCodeIsPlainNotFound(t *testing.T) {
	ts, _ := newTestServer(t, testToken)

	for _, path := range []string{"/x/nope00", "/x/", "/x/a/b"} {
		resp := do(t, "GET", ts.URL+path, "", "")
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Errorf("%s: Content-Type %q, want text/plain", path, ct)
		}
		if loc := resp.Header.Get("Location"); loc != "" {
			t.Errorf("%s: Location %q, want none", path, loc)
		}
	}
}

func TestLinksSurviveRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shrt.db")

	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ts, _ := serveStore(t, store, testToken)
	link := create(t, ts, "https://example.com/kept")
	ts.Close()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	ts, _ = serveStore(t, openTestStore(t, path), testToken)
	resp := do(t, "GET", ts.URL+"/x/"+link.Code, "", "")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != link.URL {
		t.Errorf("after restart: status %d, Location %q; want 302 to %q",
			resp.StatusCode, resp.Header.Get("Location"), link.URL)
	}
}

func TestHealthz(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "shrt.db"))
	ts, _ := serveStore(t, store, testToken)

	resp := do(t, "GET", ts.URL+"/healthz", "", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status %d, want 200", resp.StatusCode)
	}

	store.Close()
	resp = do(t, "GET", ts.URL+"/healthz", "", "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("with the database closed: status %d, want 503", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "sql") {
		t.Errorf("body %q leaks the database error", body)
	}
}
