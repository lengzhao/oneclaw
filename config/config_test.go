package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
)

func TestMerge_projectOverridesUser(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, memory.DotDir)
	projDir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`
model: from-user
openai:
  api_key: user-key
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`
model: from-project
`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(LoadOptions{Home: home, Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	if r.ChatModel() != "from-project" {
		t.Fatalf("model: got %q want from-project", r.ChatModel())
	}
	t.Setenv("OPENAI_API_KEY", "")
	if r.apiKeyResolved() != "user-key" {
		t.Fatalf("api key: got %q want user-key", r.apiKeyResolved())
	}
}

func TestMerge_explicitHighest(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	projDir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`model: proj`), 0o644); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(t.TempDir(), "extra.yaml")
	if err := os.WriteFile(explicit, []byte(`model: extra-layer`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(LoadOptions{Home: home, Cwd: cwd, ExplicitPath: explicit})
	if err != nil {
		t.Fatal(err)
	}
	if r.ChatModel() != "extra-layer" {
		t.Fatalf("model: got %q", r.ChatModel())
	}
}

func TestLoad_explicitMissing(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	_, err := Load(LoadOptions{Home: home, Cwd: cwd, ExplicitPath: "/nonexistent/oneclaw-config.yaml"})
	if err == nil {
		t.Fatal("expected error")
	}
}
