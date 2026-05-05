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

func TestE2E_contract_writeFile_memoryPath(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	instr := filepath.Join(tmp, "instruction")
	if err := os.MkdirAll(instr, 0o755); err != nil {
		t.Fatal(err)
	}
	mm := memory.MonthUTC(time.Now().UTC())
	rel := "memory/" + mm + "/contract-note.md"

	tool, err := builtin.InferWriteFileScoped(ws, instr, "")
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

func TestE2E_contract_writeFile_skillPath(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	root := filepath.Join(tmp, "user-data")

	tool, err := builtin.InferWriteFileScoped(ws, "", root)
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

func TestE2E_contract_memoryPath_writeThenReadRoundTrip(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	instr := filepath.Join(tmp, "instruction")
	if err := os.MkdirAll(instr, 0o755); err != nil {
		t.Fatal(err)
	}
	mm := memory.MonthUTC(time.Now().UTC())
	rel := "memory/" + mm + "/roundtrip.md"
	body := "extract TOKEN_MEM_R91\n"

	wtool, err := builtin.InferWriteFileScoped(ws, instr, "")
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

	rtool, err := builtin.InferReadFileScoped(ws, instr, "")
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
		t.Fatalf("read_file memory path: want %q got %q", body, got)
	}
}

func TestE2E_contract_writeFile_appendAndReplace(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")

	wtool, err := builtin.InferWriteFileScoped(ws, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wtool.InvokableRun(ctx, `{"path":"note.md","content":"alpha\n"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := wtool.InvokableRun(ctx, `{"path":"note.md","operation":"append","content":"beta\n"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := wtool.InvokableRun(ctx, `{"path":"note.md","operation":"replace","old_text":"alpha\n","content":"gamma\n"}`); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(ws, "note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "gamma\nbeta\n" {
		t.Fatalf("file content: %q", b)
	}
}

func TestE2E_contract_writeFile_allowsAnyMemoryMonth(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	instr := filepath.Join(tmp, "instruction")
	if err := os.MkdirAll(instr, 0o755); err != nil {
		t.Fatal(err)
	}

	wtool, err := builtin.InferWriteFileScoped(ws, instr, "")
	if err != nil {
		t.Fatal(err)
	}
	wargs, err := json.Marshal(map[string]string{"path": "memory/1999-01/old.md", "content": "body\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wtool.InvokableRun(ctx, string(wargs)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(instr, "memory", "1999-01", "old.md")); err != nil {
		t.Fatal(err)
	}
}

func TestE2E_contract_writeFile_instructionCoreFiles(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	instr := filepath.Join(tmp, "instruction")
	if err := os.MkdirAll(instr, 0o755); err != nil {
		t.Fatal(err)
	}
	w, err := builtin.InferWriteFileScoped(ws, instr, "")
	if err != nil {
		t.Fatal(err)
	}
	r, err := builtin.InferReadFileScoped(ws, instr, "")
	if err != nil {
		t.Fatal(err)
	}

	wargs, _ := json.Marshal(map[string]string{"path": "MEMORY.md", "content": "alpha\n"})
	if _, err := w.InvokableRun(ctx, string(wargs)); err != nil {
		t.Fatal(err)
	}
	aargs, _ := json.Marshal(map[string]string{"path": "memory.md", "operation": "append", "content": "beta\n"})
	if _, err := w.InvokableRun(ctx, string(aargs)); err != nil {
		t.Fatal(err)
	}
	rargs, _ := json.Marshal(map[string]string{"path": "MEMORY.md"})
	got, err := r.InvokableRun(ctx, string(rargs))
	if err != nil {
		t.Fatal(err)
	}
	if got != "alpha\nbeta\n" {
		t.Fatalf("MEMORY.md content: %q", got)
	}
	if _, err := w.InvokableRun(ctx, `{"path":"SOUL.md","operation":"replace","old_text":"missing","content":"x"}`); err == nil {
		t.Fatal("expected replace error for missing old_text")
	}
	uargs, _ := json.Marshal(map[string]string{"path": "USER.md", "content": "profile\n"})
	if _, err := w.InvokableRun(ctx, string(uargs)); err != nil {
		t.Fatal(err)
	}
	gotUser, err := r.InvokableRun(ctx, `{"path":"user.md"}`)
	if err != nil {
		t.Fatal(err)
	}
	if gotUser != "profile\n" {
		t.Fatalf("USER.md content: %q", gotUser)
	}
}
