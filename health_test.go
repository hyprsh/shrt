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
	if err := CheckHealth(context.Background(), u.Host); err != nil {
		t.Errorf("health: %v", err)
	}
}

func TestCheckHealthFailsOnAnythingButOK(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	if err := CheckHealth(context.Background(), u.Host); err == nil {
		t.Error("health succeeded on a 404")
	}
}

func TestCheckHealthFailsWhenNothingAnswers(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	u, _ := url.Parse(ts.URL)
	ts.Close()
	if err := CheckHealth(context.Background(), u.Host); err == nil {
		t.Error("health succeeded with nothing listening")
	}
}

func TestHealthURLReachesLoopbackForAnUnspecifiedAddress(t *testing.T) {
	for addr, want := range map[string]string{
		"0.0.0.0:8080":   "http://127.0.0.1:8080/healthz",
		"[::]:8080":      "http://127.0.0.1:8080/healthz",
		":8080":          "http://127.0.0.1:8080/healthz",
		"[::1]:9123":     "http://[::1]:9123/healthz",
		"192.0.2.7:8080": "http://192.0.2.7:8080/healthz",
	} {
		if got := healthURL(addr); got != want {
			t.Errorf("healthURL(%q) = %q, want %q", addr, got, want)
		}
	}
}
