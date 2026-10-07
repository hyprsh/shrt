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

func post(t *testing.T, ts *httptest.Server, body string) *http.Response {
	t.Helper()
	return do(t, "POST", ts.URL+"/api/links", "Bearer "+testToken, body)
}

func create(t *testing.T, ts *httptest.Server, url string) Link {
	t.Helper()
	return wantCreated(t, post(t, ts, `{"url": "`+url+`"}`))
}

func createNamed(t *testing.T, ts *httptest.Server, url, name string) Link {
	t.Helper()
	return wantCreated(t, post(t, ts, `{"url": "`+url+`", "name": "`+name+`"}`))
}

func wantCreated(t *testing.T, resp *http.Response) Link {
	t.Helper()
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
		"unknown field": `{"url": "https://example.com", "expires": "2027-01-01"}`,
		"trailing data": `{"url": "https://example.com"} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp := do(t, "POST", ts.URL+"/api/links", "Bearer "+testToken, body)
			wantError(t, resp, http.StatusBadRequest)
		})
	}
}

func TestCreateLinkWithName(t *testing.T) {
	ts, _ := newTestServer(t, testToken)

	for _, name := range []string{"ferien", "ferien-2026", "42", "-", strings.Repeat("a", maxNameLength)} {
		link := createNamed(t, ts, "https://example.com/"+name, name)
		want := Link{
			Code:      name,
			ShortURL:  "https://hypr.sh/x/" + name,
			URL:       "https://example.com/" + name,
			CreatedAt: testNow,
		}
		if link != want {
			t.Errorf("link %+v, want %+v", link, want)
		}
		resp := do(t, "GET", ts.URL+"/x/"+name, "", "")
		if got := resp.Header.Get("Location"); resp.StatusCode != http.StatusFound || got != want.URL {
			t.Errorf("/x/%s: status %d, Location %q; want 302 to %q", name, resp.StatusCode, got, want.URL)
		}
	}
}

func TestCreateLinkRejectsBadNames(t *testing.T) {
	ts, _ := newTestServer(t, testToken)

	for name, value := range map[string]string{
		"empty":       `""`,
		"upper case":  `"Ferien"`,
		"space":       `"a b"`,
		"underscore":  `"a_b"`,
		"dot":         `"a.b"`,
		"slash":       `"a/b"`,
		"umlaut":      `"ferien-über"`,
		"too long":    `"` + strings.Repeat("a", maxNameLength+1) + `"`,
		"not text":    `42`,
		"percent":     `"a%20b"`,
		"query":       `"a?b"`,
		"trailing nl": `"ferien\n"`,
	} {
		t.Run(name, func(t *testing.T) {
			resp := post(t, ts, `{"url": "https://example.com", "name": `+value+`}`)
			wantError(t, resp, http.StatusBadRequest)
		})
	}
}

func TestCreateLinkWithoutNameAcceptsNull(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	resp := post(t, ts, `{"url": "https://example.com", "name": null}`)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status %d, want 201", resp.StatusCode)
	}
}

func TestCreateLinkNameTaken(t *testing.T) {
	ts, srv := newTestServer(t, testToken)
	createNamed(t, ts, "https://example.com/first", "ferien")
	srv.newCode = func() string { return "abcdef" }
	create(t, ts, "https://example.com/generated")

	for _, name := range []string{"ferien", "abcdef"} {
		resp := post(t, ts, `{"url": "https://example.com/second", "name": "`+name+`"}`)
		wantError(t, resp, http.StatusConflict)
	}
	// The same URL under a taken name is still a conflict.
	resp := post(t, ts, `{"url": "https://example.com/first", "name": "ferien"}`)
	wantError(t, resp, http.StatusConflict)

	// Neither link is overwritten.
	for code, want := range map[string]string{
		"ferien": "https://example.com/first",
		"abcdef": "https://example.com/generated",
	} {
		resp := do(t, "GET", ts.URL+"/x/"+code, "", "")
		if got := resp.Header.Get("Location"); got != want {
			t.Errorf("/x/%s: Location %q, want %q", code, got, want)
		}
	}
}

func TestCreateLinkReturnsExistingGeneratedCode(t *testing.T) {
	ts, srv := newTestServer(t, testToken)
	first := create(t, ts, "https://example.com/page")

	srv.now = func() time.Time { return testNow.Add(time.Hour) }
	resp := post(t, ts, `{"url": "https://example.com/page"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200", resp.StatusCode)
	}
	var again Link
	decode(t, resp, &again)
	if again != first {
		t.Errorf("link %+v, want the first one %+v", again, first)
	}

	// The match is exact: a different URL gets a code of its own.
	other := create(t, ts, "https://example.com/page/")
	if other.Code == first.Code {
		t.Errorf("https://example.com/page/ got %q too", other.Code)
	}
}

func TestCreateLinkIgnoresChosenNamesForSameURL(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	const url = "https://example.com/page"
	createNamed(t, ts, url, "ferien")

	// A named link doesn't count as the URL's generated code...
	generated := create(t, ts, url)
	if generated.Code == "ferien" {
		t.Fatalf("got the chosen name back")
	}
	// ...and a name always makes its own link, even with a generated code.
	createNamed(t, ts, url, "urlaub")

	resp := post(t, ts, `{"url": "`+url+`"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200", resp.StatusCode)
	}
	var again Link
	decode(t, resp, &again)
	if again.Code != generated.Code {
		t.Errorf("code %q, want %q", again.Code, generated.Code)
	}
}

func TestCreateLinkURLLength(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	prefix := "https://example.com/"

	longest := prefix + strings.Repeat("a", maxURLLength-len(prefix))
	create(t, ts, longest)

	resp := post(t, ts, `{"url": "`+longest+`b"}`)
	wantError(t, resp, http.StatusBadRequest)

	// The limit counts characters, not bytes.
	create(t, ts, prefix+strings.Repeat("ü", maxURLLength-len(prefix)))
}

func TestCreateLinkRejectsURLsToItself(t *testing.T) {
	ts, _ := newTestServer(t, testToken)

	for _, url := range []string{
		"https://hypr.sh/x/k3P9qa",
		"http://hypr.sh/x/k3P9qa",
		"https://HYPR.sh/x/k3P9qa",
		"https://hypr.sh./x/k3P9qa",
		"https://hypr.sh:443/x/k3P9qa",
		"https://user@hypr.sh/x/k3P9qa",
		"https://hypr.sh/x/k3P9qa?q=1#frag",
		"https://hypr.sh/x/",
		"https://hypr.sh/x",
		"https://hypr.sh//x/k3P9qa",
		"https://hypr.sh/%78/k3P9qa",
		"https://hypr.sh/a/../x/k3P9qa",
		"https://hypr.sh/./x/k3P9qa",
	} {
		t.Run(url, func(t *testing.T) {
			resp := post(t, ts, `{"url": "`+url+`"}`)
			wantError(t, resp, http.StatusBadRequest)
		})
	}

	for _, url := range []string{
		"https://hypr.sh",
		"https://hypr.sh/",
		"https://hypr.sh/xy/k3P9qa",
		"https://hypr.sh/docs/x/k3P9qa",
		"https://www.hypr.sh/x/k3P9qa",
		"https://shrt.internal.hypr.sh/api/links",
		"https://example.com/x/k3P9qa",
	} {
		resp := post(t, ts, `{"url": "`+url+`"}`)
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("%s: status %d, want 201", url, resp.StatusCode)
		}
	}
}

func TestCreateLinkRejectsURLsToItselfUnderBasePath(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "shrt.db"))
	ts := httptest.NewServer(NewServer(store, "http://localhost:8080/shrt", testToken))
	t.Cleanup(ts.Close)

	resp := post(t, ts, `{"url": "http://localhost:8080/shrt/x/k3P9qa"}`)
	wantError(t, resp, http.StatusBadRequest)
	create(t, ts, "http://localhost:8080/x/k3P9qa")
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

func TestListLinksNewestFirst(t *testing.T) {
	ts, srv := newTestServer(t, testToken)
	codes := []string{"aaaaaa", "bbbbbb", "cccccc", "dddddd"}
	// The first link is the oldest; the rest share a second and tie.
	times := []time.Time{testNow, testNow.Add(time.Minute), testNow.Add(time.Minute), testNow.Add(time.Minute)}
	var i int
	srv.newCode = func() string { return codes[i] }
	srv.now = func() time.Time { return times[i] }
	for i = range codes {
		create(t, ts, "https://example.com/"+codes[i])
	}

	resp := do(t, "GET", ts.URL+"/api/links", "Bearer "+testToken, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200", resp.StatusCode)
	}
	var got []map[string]string
	decode(t, resp, &got)
	want := []string{"dddddd", "cccccc", "bbbbbb", "aaaaaa"}
	if len(got) != len(want) {
		t.Fatalf("%d links, want %d: %v", len(got), len(want), got)
	}
	created := map[string]time.Time{"aaaaaa": times[0], "bbbbbb": times[1], "cccccc": times[2], "dddddd": times[3]}
	for i, code := range want {
		entry := map[string]string{
			"code":       code,
			"short_url":  "https://hypr.sh/x/" + code,
			"url":        "https://example.com/" + code,
			"created_at": created[code].Format(time.RFC3339),
		}
		for k, v := range entry {
			if got[i][k] != v {
				t.Errorf("link %d: %s %q, want %q", i, k, got[i][k], v)
			}
		}
	}
}

func TestListLinksEmptyIsEmptyList(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	resp := do(t, "GET", ts.URL+"/api/links", "Bearer "+testToken, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "[]" {
		t.Errorf("body %q, want []", body)
	}
}

func TestShowLink(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	link := create(t, ts, "https://example.com/some/page")

	resp := do(t, "GET", ts.URL+"/api/links/"+link.Code, "Bearer "+testToken, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200", resp.StatusCode)
	}
	var got Link
	decode(t, resp, &got)
	if got != link {
		t.Errorf("got %+v, want %+v", got, link)
	}
}

func TestShowLinkUnknownIsNotFound(t *testing.T) {
	ts, srv := newTestServer(t, testToken)
	srv.newCode = func() string { return "AbCdEf" }
	create(t, ts, "https://example.com")

	for _, code := range []string{"nope00", "abcdef"} {
		resp := do(t, "GET", ts.URL+"/api/links/"+code, "Bearer "+testToken, "")
		wantError(t, resp, http.StatusNotFound)
	}
}

func TestDeleteLink(t *testing.T) {
	ts, srv := newTestServer(t, testToken)
	srv.newCode = func() string { return "ferien" }
	link := create(t, ts, "https://example.com/first")

	resp := do(t, "DELETE", ts.URL+"/api/links/"+link.Code, "Bearer "+testToken, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d, want 204", resp.StatusCode)
	}
	if body, _ := io.ReadAll(resp.Body); len(body) != 0 {
		t.Errorf("body %q, want none", body)
	}

	if resp := do(t, "GET", ts.URL+"/x/"+link.Code, "", ""); resp.StatusCode != http.StatusNotFound {
		t.Errorf("redirect after delete: status %d, want 404", resp.StatusCode)
	}
	if resp := do(t, "GET", ts.URL+"/api/links/"+link.Code, "Bearer "+testToken, ""); resp.StatusCode != http.StatusNotFound {
		t.Errorf("show after delete: status %d, want 404", resp.StatusCode)
	}
	resp = do(t, "GET", ts.URL+"/api/links", "Bearer "+testToken, "")
	var links []Link
	decode(t, resp, &links)
	if len(links) != 0 {
		t.Errorf("list after delete: %v, want none", links)
	}

	// The freed code can be created again, for another URL.
	again := create(t, ts, "https://example.com/second")
	if again.Code != link.Code {
		t.Errorf("code %q, want %q", again.Code, link.Code)
	}
	resp = do(t, "GET", ts.URL+"/x/"+link.Code, "", "")
	if got := resp.Header.Get("Location"); got != "https://example.com/second" {
		t.Errorf("Location %q, want https://example.com/second", got)
	}
}

func TestDeletedNameCanBeChosenAgain(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	createNamed(t, ts, "https://example.com/first", "ferien")

	resp := do(t, "DELETE", ts.URL+"/api/links/ferien", "Bearer "+testToken, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: status %d, want 204", resp.StatusCode)
	}
	createNamed(t, ts, "https://example.com/second", "ferien")
	resp = do(t, "GET", ts.URL+"/x/ferien", "", "")
	if got := resp.Header.Get("Location"); got != "https://example.com/second" {
		t.Errorf("Location %q, want https://example.com/second", got)
	}
}

func TestDeletedGeneratedCodeIsNotReturnedForItsURL(t *testing.T) {
	ts, srv := newTestServer(t, testToken)
	codes := []string{"aaaaaa", "bbbbbb"}
	srv.newCode = func() string {
		code := codes[0]
		codes = codes[1:]
		return code
	}
	const url = "https://example.com/page"
	create(t, ts, url)

	resp := do(t, "DELETE", ts.URL+"/api/links/aaaaaa", "Bearer "+testToken, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: status %d, want 204", resp.StatusCode)
	}
	if again := create(t, ts, url); again.Code != "bbbbbb" {
		t.Errorf("code %q, want a new one, bbbbbb", again.Code)
	}
}

func TestDeleteLinkKeepsOthers(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	keep := create(t, ts, "https://example.com/keep")
	drop := create(t, ts, "https://example.com/drop")

	resp := do(t, "DELETE", ts.URL+"/api/links/"+drop.Code, "Bearer "+testToken, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: status %d, want 204", resp.StatusCode)
	}
	resp = do(t, "GET", ts.URL+"/x/"+keep.Code, "", "")
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status %d, want 302", resp.StatusCode)
	}
}

func TestDeleteLinkUnknownIsNotFound(t *testing.T) {
	ts, srv := newTestServer(t, testToken)
	srv.newCode = func() string { return "AbCdEf" }
	link := create(t, ts, "https://example.com")

	for _, code := range []string{"nope00", "abcdef"} {
		resp := do(t, "DELETE", ts.URL+"/api/links/"+code, "Bearer "+testToken, "")
		wantError(t, resp, http.StatusNotFound)
	}
	if resp := do(t, "GET", ts.URL+"/x/"+link.Code, "", ""); resp.StatusCode != http.StatusFound {
		t.Errorf("redirect: status %d, want 302", resp.StatusCode)
	}
}

func TestListShowDeleteRefuseMissingOrWrongToken(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	link := create(t, ts, "https://example.com")

	for _, call := range []string{"GET /api/links", "GET /api/links/" + link.Code, "DELETE /api/links/" + link.Code} {
		method, path, _ := strings.Cut(call, " ")
		for _, auth := range []string{"", "Bearer not-the-token"} {
			resp := do(t, method, ts.URL+path, auth, "")
			wantError(t, resp, http.StatusUnauthorized)
		}
	}
	// The refused delete left the link in place.
	if resp := do(t, "GET", ts.URL+"/x/"+link.Code, "", ""); resp.StatusCode != http.StatusFound {
		t.Errorf("redirect: status %d, want 302", resp.StatusCode)
	}
}
