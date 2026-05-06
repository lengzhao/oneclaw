package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInferReadSkill_defaultSkillFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("hello skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool, err := InferReadSkill(root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := tool.InvokableRun(ctx, `{"skill_id":"demo-skill"}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "hello skill" {
		t.Fatalf("got %q", out)
	}
}

func TestInferReadSkill_blocksTraversal(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	tool, err := InferReadSkill(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tool.InvokableRun(ctx, `{"skill_id":"demo-skill","path":"../x.md"}`)
	if err == nil {
		t.Fatal("expected traversal error")
	}
}
