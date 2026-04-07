package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
)

func boolPtr(b bool) *bool { return &b }

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

func TestMerge_mcpEnabled(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	projDir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`
mcp:
  enabled: true
  max_inline_text_runes: 1234
  servers:
    s1:
      enabled: true
      command: echo
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(LoadOptions{Home: home, Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	if !r.MCPEnabled() {
		t.Fatal("MCPEnabled: want true")
	}
	m := r.MCP()
	if m.MaxInlineTextRunes != 1234 {
		t.Fatalf("MaxInlineTextRunes: got %d", m.MaxInlineTextRunes)
	}
	if len(m.Servers) != 1 || !m.Servers["s1"].Enabled || m.Servers["s1"].Command != "echo" {
		t.Fatalf("servers: %#v", m.Servers)
	}
}

func TestMergeMCP_laterDisables(t *testing.T) {
	var dst MCPFile
	mergeMCP(&dst, MCPFile{Enabled: boolPtr(true)})
	mergeMCP(&dst, MCPFile{Enabled: boolPtr(false)})
	var f File
	f.MCP = dst
	res := &Resolved{merged: f}
	if res.MCPEnabled() {
		t.Fatal("expected MCP off after false layer")
	}
}
