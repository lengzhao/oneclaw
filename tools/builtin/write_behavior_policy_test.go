package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/toolctx"
)

func TestWriteBehaviorPolicyProjectSkill(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	tctx.HomeDir = home

	payload, _ := json.Marshal(map[string]string{
		"target":    "skill",
		"rule_name": "my-skill",
		"content":   "# my-skill\n",
	})
	var tool WriteBehaviorPolicyTool
	out, err := tool.Execute(context.Background(), payload, tctx)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(cwd, memory.DotDir, "skills", "my-skill", skillEntryFile)
	if !strings.Contains(out, wantPath) {
		t.Fatalf("got %q, want path %q", out, wantPath)
	}
	b, err := os.ReadFile(wantPath)
	if err != nil || string(b) != "# my-skill\n" {
		t.Fatalf("read back: %v %q", err, string(b))
	}
}

func TestWriteBehaviorPolicyMemory(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	tctx.HomeDir = home
	payload, _ := json.Marshal(map[string]string{
		"target":  "memory",
		"content": "# MEMORY\n\n- ok\n",
	})
	var tool WriteBehaviorPolicyTool
	if _, err := tool.Execute(context.Background(), payload, tctx); err != nil {
		t.Fatal(err)
	}
	want := memory.ProjectMemoryMdPath(cwd)
	b, err := os.ReadFile(want)
	if err != nil || string(b) != "# MEMORY\n\n- ok\n" {
		t.Fatalf("read back: %v %q", err, string(b))
	}
}

func TestWriteBehaviorPolicyAgentMdRejectsRuleName(t *testing.T) {
	cwd := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	payload, _ := json.Marshal(map[string]string{
		"target":    "agent_md",
		"rule_name": "x",
		"content":   "# x\n",
	})
	var tool WriteBehaviorPolicyTool
	if _, err := tool.Execute(context.Background(), payload, tctx); err == nil {
		t.Fatal("expected error when rule_name set for agent_md")
	}
}

func TestWriteBehaviorPolicyRejectsEscape(t *testing.T) {
	cwd := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	payload, _ := json.Marshal(map[string]string{
		"target":    "skill",
		"rule_name": "../x",
		"content":   "nope",
	})
	var tool WriteBehaviorPolicyTool
	if _, err := tool.Execute(context.Background(), payload, tctx); err == nil {
		t.Fatal("expected error for path escape")
	}
}
