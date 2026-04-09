package subagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAgentFile_frontmatter(t *testing.T) {
	raw := []byte(`---
agent_type: demo-agent
description: Demo
tools:
  - read_file
max_turns: 5
model: " gpt-4o-mini "
---
You are a demo.
`)
	d, err := ParseAgentFile("/tmp/x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.AgentType != "demo-agent" || d.MaxTurns != 5 || len(d.Tools) != 1 || d.Model != "gpt-4o-mini" {
		t.Fatalf("def: %+v", d)
	}
	if d.SystemPrompt != "You are a demo." {
		t.Fatalf("body: %q", d.SystemPrompt)
	}
}

func TestLoadCatalog_userOverridesBuiltin(t *testing.T) {
	cwd := t.TempDir()
	agents := filepath.Join(cwd, ".oneclaw", "agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := []byte(`---
agent_type: explore
description: overridden
tools:
  - read_file
---
Custom explore body.
`)
	if err := os.WriteFile(filepath.Join(agents, "explore.md"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	cat := LoadCatalog(cwd)
	d, ok := cat.Get("explore")
	if !ok {
		t.Fatal("missing explore")
	}
	if d.SystemPrompt != "Custom explore body." {
		t.Fatalf("system: %q", d.SystemPrompt)
	}
}

func TestPromptCatalogLines_byteBudget(t *testing.T) {
	cat := LoadCatalog(t.TempDir())
	lines := cat.PromptCatalogLines(500)
	if len(lines) < 2 {
		t.Fatalf("expected at least built-in agents, got %d lines", len(lines))
	}
	tight := cat.PromptCatalogLines(30)
	if len(tight) > len(lines) {
		t.Fatalf("tighter budget should not yield more lines")
	}
}
