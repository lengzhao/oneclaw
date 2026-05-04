package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalEnv_mapsAlternateKeys(t *testing.T) {
	for _, tc := range []struct {
		in       string
		want     string
		wantBool bool
	}{
		{"base_url", EnvBaseURL, true},
		{"BASE_URL", EnvBaseURL, true},
		{"API_KEY", EnvAPIKey, true},
		{"default_model", EnvDefaultModel, true},
		{"ONECLAW_E2E_DEFAULT_MODEL", EnvDefaultModel, true},
		{"FOOBAR", "", false},
	} {
		got, ok := canonicalEnv(tc.in)
		if ok != tc.wantBool || got != tc.want {
			t.Fatalf("canonicalEnv(%q) = (%q,%v) want (%q,%v)", tc.in, got, ok, tc.want, tc.wantBool)
		}
	}
}

func TestLoad_respectsExistingEnv(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(tmp, []byte("API_KEY=from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvAPIKey, "preset")
	if err := Load(tmp); err != nil {
		t.Fatal(err)
	}
	if os.Getenv(EnvAPIKey) != "preset" {
		t.Fatalf("should not override existing env: got %q", os.Getenv(EnvAPIKey))
	}
}

func TestLoad_setsWhenUnset(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), ".env")
	content := "BASE_URL=https://example.invalid/v1\nAPI_KEY=secret\nDEFAULT_MODEL=m-m\n"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvBaseURL, "")
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvDefaultModel, "")

	if err := Load(tmp); err != nil {
		t.Fatal(err)
	}
	if os.Getenv(EnvBaseURL) != "https://example.invalid/v1" || os.Getenv(EnvAPIKey) != "secret" || os.Getenv(EnvDefaultModel) != "m-m" {
		t.Fatalf("got base=%q key=%q dm=%q", os.Getenv(EnvBaseURL), os.Getenv(EnvAPIKey), os.Getenv(EnvDefaultModel))
	}
}
