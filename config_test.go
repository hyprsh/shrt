package shrt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestConfigFromEnvDefaults(t *testing.T) {
	cfg, err := ConfigFromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Addr: "127.0.0.1:8080", BaseURL: "https://sh.hypr.sh", DBPath: "shrt.db"}
	if cfg != want {
		t.Errorf("config %+v, want %+v", cfg, want)
	}
}

func TestConfigFromEnv(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(tokenFile, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := ConfigFromEnv(env(map[string]string{
		"SHRT_ADDRESS":    "::1",
		"SHRT_PORT":       "9123",
		"SHRT_BASE_URL":   "https://short.example/",
		"SHRT_TOKEN_FILE": tokenFile,
		"SHRT_DB_PATH":    "/var/lib/shrt/shrt.db",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Addr:    "[::1]:9123",
		BaseURL: "https://short.example",
		Token:   "s3cret",
		DBPath:  "/var/lib/shrt/shrt.db",
	}
	if cfg != want {
		t.Errorf("config %+v, want %+v", cfg, want)
	}
}

func TestConfigFromEnvTokenDirectly(t *testing.T) {
	cfg, err := ConfigFromEnv(env(map[string]string{"SHRT_TOKEN": " s3cret\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "s3cret" {
		t.Errorf("token %q, want s3cret", cfg.Token)
	}
}

func TestConfigFromEnvBothTokensIsAnError(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(tokenFile, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ConfigFromEnv(env(map[string]string{"SHRT_TOKEN": "x", "SHRT_TOKEN_FILE": tokenFile}))
	if err == nil || !strings.Contains(err.Error(), "one way") {
		t.Errorf("error %v, want one saying to give the token one way", err)
	}
}

func TestConfigFromEnvEmptyTokenFileIsAnError(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(tokenFile, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ConfigFromEnv(env(map[string]string{"SHRT_TOKEN_FILE": tokenFile}))
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("error %v, want one saying the file is empty", err)
	}
}

func TestCheckEnv(t *testing.T) {
	known := []string{"SHRT_ADDRESS=0.0.0.0", "SHRT_PORT=8080", "SHRT_BASE_URL=https://x", "SHRT_TOKEN=a", "SHRT_TOKEN_FILE=/f", "SHRT_DB_PATH=/db", "PATH=/bin", "SHRTX=1"}
	if err := CheckEnv(known); err != nil {
		t.Errorf("known variables: %v", err)
	}
	err := CheckEnv(append(known, "SHRT_TOKEN_FIEL=/f", "SHRT_PROT=1"))
	if err == nil {
		t.Fatal("want an error for unknown variables")
	}
	for _, name := range []string{"SHRT_TOKEN_FIEL", "SHRT_PROT"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

func TestConfigFromEnvErrors(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"missing token file": {"SHRT_TOKEN_FILE": filepath.Join(t.TempDir(), "nope")},
		"port not a number":  {"SHRT_PORT": "http"},
		"port out of range":  {"SHRT_PORT": "70000"},
		"base URL relative":  {"SHRT_BASE_URL": "hypr.sh"},
		"base URL scheme":    {"SHRT_BASE_URL": "ftp://hypr.sh"},
		"base URL query":     {"SHRT_BASE_URL": "https://hypr.sh/?a=b"},
	} {
		t.Run(name, func(t *testing.T) {
			if cfg, err := ConfigFromEnv(env(vars)); err == nil {
				t.Errorf("config %+v, want an error", cfg)
			}
		})
	}
}
