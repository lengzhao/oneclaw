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
	wantPath := filepath.Join(cwd, "skills", "my-skill", skillEntryFile)
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
	want := filepath.Join(memory.ProjectMemoryDir(cwd), "MEMORY.md")
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

func TestWriteBehaviorPolicyAgentMemoryProject(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	tctx.HomeDir = home
	payload, _ := json.Marshal(map[string]string{
		"target":     "agent_memory",
		"scope":      "local",
		"agent_type": "reviewer",
		"content":    "# agent proj\n",
	})
	var tool WriteBehaviorPolicyTool
	out, err := tool.Execute(context.Background(), payload, tctx)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(cwd, "agent-memory", "reviewer", "MEMORY.md")
	if !strings.Contains(out, wantPath) {
		t.Fatalf("got %q, want path %q", out, wantPath)
	}
}

func TestWriteBehaviorPolicyAgentMemoryRequiresAgentType(t *testing.T) {
	cwd := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	payload, _ := json.Marshal(map[string]string{
		"target": "agent_memory",
		"scope":  "public",
		"content": "# missing type\n",
	})
	var tool WriteBehaviorPolicyTool
	if _, err := tool.Execute(context.Background(), payload, tctx); err == nil {
		t.Fatal("expected error when agent_type missing for agent_memory")
	}
}

func TestWriteBehaviorPolicyAgentMemoryDefaultsToLocalInIsolatedSession(t *testing.T) {
	home := t.TempDir()
	userRoot := filepath.Join(home, memory.DotDir)
	sessionRoot := filepath.Join(userRoot, "sessions", "s1")
	cwd := filepath.Join(sessionRoot, memory.IMWorkspaceDirName)
	tctx := toolctx.New(cwd, context.Background())
	tctx.HomeDir = home
	tctx.HostDataRoot = userRoot
	tctx.WorkspaceFlat = true
	tctx.InstructionRoot = sessionRoot

	payload, _ := json.Marshal(map[string]string{
		"target":     "agent_memory",
		"agent_type": "reviewer",
		"content":    "# isolated local\n",
	})
	var tool WriteBehaviorPolicyTool
	out, err := tool.Execute(context.Background(), payload, tctx)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(sessionRoot, "agent-memory", "reviewer", "MEMORY.md")
	if !strings.Contains(out, wantPath) {
		t.Fatalf("got %q, want path %q", out, wantPath)
	}
}

func TestWriteBehaviorPolicyAgentMemoryRequiresScopeWhenAmbiguous(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	tctx.HomeDir = home
	payload, _ := json.Marshal(map[string]string{
		"target":     "agent_memory",
		"agent_type": "reviewer",
		"content":    "# missing scope\n",
	})
	var tool WriteBehaviorPolicyTool
	if _, err := tool.Execute(context.Background(), payload, tctx); err == nil {
		t.Fatal("expected error when scope omitted in ambiguous layout")
	}
}
