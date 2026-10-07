package shrt

import (
	"os"
	"path/filepath"
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
	want := Config{Addr: "127.0.0.1:8080", BaseURL: "https://hypr.sh", DBPath: "shrt.db"}
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

func TestConfigFromEnvEmptyTokenFileMeansNoToken(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(tokenFile, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := ConfigFromEnv(env(map[string]string{"SHRT_TOKEN_FILE": tokenFile}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "" {
		t.Errorf("token %q, want none", cfg.Token)
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
