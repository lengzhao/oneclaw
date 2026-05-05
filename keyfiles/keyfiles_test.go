package keyfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteThenReadBearerSecret(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "key_files", "dashscope.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := MergeWriteAPIKey(p, "sk-test"); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBearerSecret(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "sk-test" {
		t.Fatalf("got %q", got)
	}
}

func TestReadBearerSecret_schemaVersionExtra(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cred.json")
	raw := `{"schema_version":1,"provider":"alibaba","api_key":"k-from-bundle"}`
	if err := os.WriteFile(p, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBearerSecret(p)
	if err != nil || got != "k-from-bundle" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestReadBearerSecret_rawFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "plain")
	if err := os.WriteFile(p, []byte("  bare-key \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBearerSecret(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "bare-key" {
		t.Fatalf("got %q", got)
	}
}
