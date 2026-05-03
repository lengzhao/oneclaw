package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

func TestE2E_contract_writeMemoryMonth(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	instr := filepath.Join(tmp, "instruction")
	if err := os.MkdirAll(instr, 0o755); err != nil {
		t.Fatal(err)
	}
	mm := memory.MonthUTC(time.Now().UTC())
	rel := "memory/" + mm + "/contract-note.md"

	tool, err := builtin.InferWriteMemoryMonth(instr)
	if err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]string{"path": rel, "content": "e2e memory body\n"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := tool.(einotool.InvokableTool).InvokableRun(ctx, string(args))
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty tool output")
	}
	full := filepath.Join(instr, filepath.FromSlash(rel))
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "e2e memory body\n" {
		t.Fatalf("file content: %q", b)
	}
}

func TestE2E_contract_writeSkillFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	tool, err := builtin.InferWriteSkillFile(root)
	if err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]string{
		"path":    "skills/e2e-contract-skill/SKILL.md",
		"content": "---\nname: E2E Contract\n---\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.(einotool.InvokableTool).InvokableRun(ctx, string(args)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "skills", "e2e-contract-skill", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 10 {
		t.Fatalf("unexpected SKILL.md: %q", b)
	}
}
