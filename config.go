package shrt

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Config is shrt's configuration, read from environment variables.
type Config struct {
	// Addr is the host:port to listen on, from SHRT_ADDRESS (default
	// 127.0.0.1) and SHRT_PORT (default 8080).
	Addr string
	// BaseURL prefixes /<code> in a link's short_url, from SHRT_BASE_URL
	// (default https://sh.hypr.sh), without a trailing slash.
	BaseURL string
	// Token is the API bearer token, from SHRT_TOKEN or from the file
	// SHRT_TOKEN_FILE names, such as a Compose secret. Setting both is an
	// error, and so is a missing or empty file. Empty when neither is set,
	// and then every API call is refused.
	Token string
	// DBPath is the SQLite file, from SHRT_DB_PATH (default shrt.db).
	DBPath string
}

// ConfigFromEnv reads the configuration through getenv, such as os.Getenv.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	host, port, err := listenFromEnv(getenv)
	if err != nil {
		return Config{}, err
	}
	get := envDefault(getenv)

	baseURL, err := parseBaseURL(get("SHRT_BASE_URL", "https://sh.hypr.sh"))
	if err != nil {
		return Config{}, fmt.Errorf("SHRT_BASE_URL: %w", err)
	}

	token, err := readToken(getenv("SHRT_TOKEN_FILE"), getenv("SHRT_TOKEN"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		Addr:    net.JoinHostPort(host, port),
		BaseURL: baseURL,
		Token:   token,
		DBPath:  get("SHRT_DB_PATH", "shrt.db"),
	}, nil
}

// envDefault returns a lookup of key through getenv that gives fallback for an
// unset or empty variable.
func envDefault(getenv func(string) string) func(key, fallback string) string {
	return func(key, fallback string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return fallback
	}
}

// listenFromEnv reads SHRT_ADDRESS and SHRT_PORT with their defaults, for
// both the server and `shrt health`.
func listenFromEnv(getenv func(string) string) (host, port string, err error) {
	get := envDefault(getenv)
	port = get("SHRT_PORT", "8080")
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return "", "", fmt.Errorf("SHRT_PORT %q is not a port number", port)
	}
	return get("SHRT_ADDRESS", "127.0.0.1"), port, nil
}

// envVars are the SHRT_* variables shrt reads.
var envVars = []string{"SHRT_ADDRESS", "SHRT_PORT", "SHRT_BASE_URL", "SHRT_TOKEN", "SHRT_TOKEN_FILE", "SHRT_DB_PATH"}

// CheckEnv returns an error naming the SHRT_* variables in environ (as
// os.Environ gives them) that shrt doesn't read, so a typo such as
// SHRT_TOKEN_FIEL can't leave the API locked without a word.
func CheckEnv(environ []string) error {
	var unknown []string
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "SHRT_") && !slices.Contains(envVars, name) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return fmt.Errorf("unknown variables %s; shrt reads %s", strings.Join(unknown, ", "), strings.Join(envVars, ", "))
	}
	return nil
}

// readToken returns the token held by the file at path or by value, following
// tnl's convention: one of the two, never both, and a file that is missing or
// empty is an error. It returns "" when neither is given.
func readToken(path, value string) (string, error) {
	switch {
	case path != "" && value != "":
		return "", errors.New("SHRT_TOKEN_FILE and SHRT_TOKEN are both set: give the token one way")
	case path != "":
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("SHRT_TOKEN_FILE: %w", err)
		}
		token := strings.TrimSpace(string(b))
		if token == "" {
			return "", fmt.Errorf("SHRT_TOKEN_FILE: %s is empty", path)
		}
		return token, nil
	}
	return strings.TrimSpace(value), nil
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
