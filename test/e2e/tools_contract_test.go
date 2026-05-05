package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	out, err := tool.InvokableRun(ctx, string(args))
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
	if _, err := tool.InvokableRun(ctx, string(args)); err != nil {
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

func TestE2E_contract_memoryMonth_writeThenReadRoundTrip(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	instr := filepath.Join(tmp, "instruction")
	if err := os.MkdirAll(instr, 0o755); err != nil {
		t.Fatal(err)
	}
	mm := memory.MonthUTC(time.Now().UTC())
	rel := "memory/" + mm + "/roundtrip.md"
	body := "extract TOKEN_MEM_R91\n"

	wtool, err := builtin.InferWriteMemoryMonth(instr)
	if err != nil {
		t.Fatal(err)
	}
	wargs, err := json.Marshal(map[string]string{"path": rel, "content": body})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wtool.InvokableRun(ctx, string(wargs)); err != nil {
		t.Fatal(err)
	}

	rtool, err := builtin.InferReadMemoryMonth(instr)
	if err != nil {
		t.Fatal(err)
	}
	rargs, err := json.Marshal(map[string]string{"path": rel})
	if err != nil {
		t.Fatal(err)
	}
	got, err := rtool.InvokableRun(ctx, string(rargs))
	if err != nil {
		t.Fatal(err)
	}
	if got != body {
		t.Fatalf("read_memory_month: want %q got %q", body, got)
	}
}

func TestE2E_contract_memoryMonth_defaultPathWhenOmitted(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	instr := filepath.Join(tmp, "instruction")
	if err := os.MkdirAll(instr, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	rel := "memory/" + memory.MonthUTC(now) + "/" + now.Format("2006-01-02") + ".md"
	body := "default path body\n"

	wtool, err := builtin.InferWriteMemoryMonth(instr)
	if err != nil {
		t.Fatal(err)
	}
	wargs, err := json.Marshal(map[string]string{"content": body})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wtool.InvokableRun(ctx, string(wargs)); err != nil {
		t.Fatal(err)
	}

	rtool, err := builtin.InferReadMemoryMonth(instr)
	if err != nil {
		t.Fatal(err)
	}
	rargs, err := json.Marshal(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := rtool.InvokableRun(ctx, string(rargs))
	if err != nil {
		t.Fatal(err)
	}
	if got != body {
		t.Fatalf("read default path: want %q got %q", body, got)
	}

	b, err := os.ReadFile(filepath.Join(instr, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != body {
		t.Fatalf("default file content: want %q got %q", body, string(b))
	}
}

func TestE2E_contract_memoryMonth_invalidPathFallsBackToDefault(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	instr := filepath.Join(tmp, "instruction")
	if err := os.MkdirAll(instr, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	rel := "memory/" + memory.MonthUTC(now) + "/" + now.Format("2006-01-02") + ".md"
	body := "fallback path body\n"

	wtool, err := builtin.InferWriteMemoryMonth(instr)
	if err != nil {
		t.Fatal(err)
	}
	wargs, err := json.Marshal(map[string]string{"path": "invalid-path-shape", "content": body})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wtool.InvokableRun(ctx, string(wargs)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(instr, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != body {
		t.Fatalf("fallback file content: want %q got %q", body, string(b))
	}
}
