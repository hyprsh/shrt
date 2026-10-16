package shrt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCheckHealthAgainstARunningShrt(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	u, _ := url.Parse(ts.URL)
	if err := checkHealth(context.Background(), u.Hostname(), u.Port()); err != nil {
		t.Errorf("health: %v", err)
	}
}

func TestCheckHealthFailsOnAnythingButOK(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	if err := checkHealth(context.Background(), u.Hostname(), u.Port()); err == nil {
		t.Error("health succeeded on a 404")
	}
}

func TestCheckHealthFailsWhenNothingAnswers(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	u, _ := url.Parse(ts.URL)
	ts.Close()
	if err := checkHealth(context.Background(), u.Hostname(), u.Port()); err == nil {
		t.Error("health succeeded with nothing listening")
	}
}

func TestHealthURLReachesLoopbackForAnUnspecifiedHost(t *testing.T) {
	for _, tc := range []struct{ host, port, want string }{
		{"0.0.0.0", "8080", "http://127.0.0.1:8080/healthz"},
		{"::", "8080", "http://127.0.0.1:8080/healthz"},
		{"", "8080", "http://127.0.0.1:8080/healthz"},
		{"::1", "9123", "http://[::1]:9123/healthz"},
		{"192.0.2.7", "8080", "http://192.0.2.7:8080/healthz"},
	} {
		if got := healthURL(tc.host, tc.port); got != tc.want {
			t.Errorf("healthURL(%q, %q) = %q, want %q", tc.host, tc.port, got, tc.want)
		}
	}
}

func TestCheckHealthFromEnvNeedsOnlyTheAddressAndPort(t *testing.T) {
	ts, _ := newTestServer(t, testToken)
	u, _ := url.Parse(ts.URL)
	env := map[string]string{
		"SHRT_ADDRESS":    u.Hostname(),
		"SHRT_PORT":       u.Port(),
		"SHRT_TOKEN_FILE": "/nonexistent/token",
		"SHRT_BASE_URL":   "not a url",
	}
	if err := CheckHealthFromEnv(context.Background(), func(k string) string { return env[k] }); err != nil {
		t.Errorf("health: %v", err)
	}
}

func TestCheckHealthFromEnvRejectsABadPort(t *testing.T) {
	err := CheckHealthFromEnv(context.Background(), func(k string) string {
		if k == "SHRT_PORT" {
			return "http"
		}
		return ""
	})
	if err == nil {
		t.Error("health accepted SHRT_PORT=http")
	}
}
