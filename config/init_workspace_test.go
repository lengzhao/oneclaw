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
	closeLog := logx.Init("error", "text", "")
	defer closeLog()
	home := t.TempDir()
	if err := config.InitWorkspace(home, home); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(home, memory.DotDir, "config.yaml")
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
	if err := config.InitWorkspace(home, home); err != nil {
		t.Fatal(err)
	}
	raw2, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(raw2) {
		t.Fatal("config should be unchanged on second init")
	}
	memPath := filepath.Join(home, memory.DotDir, "memory", "MEMORY.md")
	if _, err := os.Stat(memPath); err != nil {
		t.Fatalf("MEMORY.md from init template: %v", err)
	}
	agentPath := filepath.Join(home, memory.DotDir, "AGENT.md")
	if _, err := os.Stat(agentPath); err != nil {
		t.Fatalf("AGENT.md from init template: %v", err)
	}
	for _, name := range []string{"MAINTAIN_SCHEDULED.md", "MAINTAIN_POST_TURN.md"} {
		p := filepath.Join(home, memory.DotDir, name)
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s from init template: %v", name, err)
		}
		if !strings.Contains(string(raw), "silent memory indexer") {
			t.Fatalf("%s: expected default maintain template content", name)
		}
	}
}

func TestInitWorkspaceMergesMissingKeys(t *testing.T) {
	closeLog := logx.Init("error", "text", "")
	defer closeLog()
	home := t.TempDir()
	dot := filepath.Join(home, memory.DotDir)
	if err := os.MkdirAll(dot, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dot, "config.yaml")
	orig := []byte("openai:\n  api_key: \"user-key\"\nmodel: keep-me\n")
	if err := os.WriteFile(cfgPath, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.InitWorkspace(home, home); err != nil {
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
	closeLog := logx.Init("error", "text", "")
	defer closeLog()
	home := t.TempDir()
	dot := filepath.Join(home, memory.DotDir)
	if err := os.MkdirAll(dot, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dot, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("openai: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.InitWorkspace(home, home); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
