package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/logx"
	"github.com/lengzhao/oneclaw/memory"
)

func TestInitWorkspaceWritesConfigAndDirs(t *testing.T) {
	logx.Init("error", "text")
	home := t.TempDir()
	cwd := t.TempDir()
	if err := config.InitWorkspace(cwd, home); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cwd, memory.DotDir, "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config: %v", err)
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 100 || raw[0] != '#' {
		t.Fatalf("unexpected config content")
	}
	// Second run: must not error and must not truncate config
	if err := config.InitWorkspace(cwd, home); err != nil {
		t.Fatal(err)
	}
	raw2, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(raw2) {
		t.Fatal("config should be unchanged on second init")
	}
}

func TestInitWorkspaceMergesMissingKeys(t *testing.T) {
	logx.Init("error", "text")
	home := t.TempDir()
	cwd := t.TempDir()
	dot := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(dot, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dot, "config.yaml")
	orig := []byte("openai:\n  api_key: \"user-key\"\nmodel: keep-me\n")
	if err := os.WriteFile(cfgPath, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.InitWorkspace(cwd, home); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"user-key", "keep-me", "clawbridge:", "budget:"} {
		if !strings.Contains(string(out), sub) {
			t.Fatalf("merged config missing %q:\n%s", sub, string(out))
		}
	}
}

func TestInitWorkspaceInvalidYAML(t *testing.T) {
	logx.Init("error", "text")
	home := t.TempDir()
	cwd := t.TempDir()
	dot := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(dot, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dot, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("openai: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.InitWorkspace(cwd, home); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
