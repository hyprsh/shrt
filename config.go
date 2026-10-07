package shrt

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config is shrt's configuration, read from environment variables set by
// nixos-config.
type Config struct {
	// Addr is the host:port to listen on, from SHRT_ADDRESS (default
	// 127.0.0.1) and SHRT_PORT (default 8080).
	Addr string
	// BaseURL prefixes /x/<code> in a link's short_url, from SHRT_BASE_URL
	// (default https://hypr.sh), without a trailing slash.
	BaseURL string
	// Token is the API bearer token, read from the file SHRT_TOKEN_FILE
	// names, as systemd's LoadCredential provides it. Empty when the
	// variable is unset or the file is blank, and then every API call is
	// refused.
	Token string
	// DBPath is the SQLite file, from SHRT_DB_PATH (default shrt.db).
	DBPath string
}

// ConfigFromEnv reads the configuration through getenv, such as os.Getenv.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	get := func(key, fallback string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return fallback
	}

	port := get("SHRT_PORT", "8080")
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("SHRT_PORT %q is not a port number", port)
	}

	baseURL, err := parseBaseURL(get("SHRT_BASE_URL", "https://hypr.sh"))
	if err != nil {
		return Config{}, fmt.Errorf("SHRT_BASE_URL: %w", err)
	}

	var token string
	if path := getenv("SHRT_TOKEN_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("SHRT_TOKEN_FILE: %w", err)
		}
		token = strings.TrimSpace(string(b))
	}

	return Config{
		Addr:    net.JoinHostPort(get("SHRT_ADDRESS", "127.0.0.1"), port),
		BaseURL: baseURL,
		Token:   token,
		DBPath:  get("SHRT_DB_PATH", "shrt.db"),
	}, nil
}

func parseBaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if !isAbsoluteHTTPURL(u) {
		return "", fmt.Errorf("%q is not an absolute http:// or https:// URL", raw)
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", errors.New("a base URL has no query, fragment or user")
	}
	return strings.TrimRight(raw, "/"), nil
}
