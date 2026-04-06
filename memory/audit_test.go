package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendMemoryAudit_WritesLine(t *testing.T) {
	t.Setenv("ONCLAW_DISABLE_MEMORY_AUDIT", "")
	cwd := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	lay := DefaultLayout(cwd, home)
	memFile := filepath.Join(lay.Project, "topic.md")
	_ = os.MkdirAll(filepath.Dir(memFile), 0o755)
	content := []byte("hello audit\n")
	AppendMemoryAudit(lay, memFile, "write_file", content)

	auditPath := filepath.Join(cwd, DotDir, "audit", "memory-write.jsonl")
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var rec auditRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("json: %v\n%s", err, raw)
	}
	if rec.Source != "write_file" || rec.Path != memFile || rec.Bytes != len(content) {
		t.Fatalf("record: %+v", rec)
	}
	if len(rec.SHA256) != 64 {
		t.Fatalf("sha256: %q", rec.SHA256)
	}
}

func TestAppendMemoryAudit_SkipsOutsideRoots(t *testing.T) {
	t.Setenv("ONCLAW_DISABLE_MEMORY_AUDIT", "")
	cwd := t.TempDir()
	out := filepath.Join(cwd, "outside.txt")
	lay := DefaultLayout(cwd, t.TempDir())
	AppendMemoryAudit(lay, out, "write_file", []byte("x"))
	auditPath := filepath.Join(cwd, DotDir, "audit", "memory-write.jsonl")
	if _, err := os.Stat(auditPath); err == nil {
		t.Fatal("expected no audit file")
	}
}

func TestAppendMemoryAudit_Disabled(t *testing.T) {
	t.Setenv("ONCLAW_DISABLE_MEMORY_AUDIT", "1")
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	memFile := filepath.Join(lay.Project, "t.md")
	_ = os.MkdirAll(filepath.Dir(memFile), 0o755)
	AppendMemoryAudit(lay, memFile, "x", []byte("y"))
	auditPath := filepath.Join(cwd, DotDir, "audit", "memory-write.jsonl")
	if _, err := os.Stat(auditPath); err == nil {
		t.Fatal("expected no audit file when disabled")
	}
}

func TestAppendMemoryAudit_ProjectRulesFile(t *testing.T) {
	t.Setenv("ONCLAW_DISABLE_MEMORY_AUDIT", "")
	cwd := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	lay := DefaultLayout(cwd, home)
	rulesDir := filepath.Join(cwd, DotDir, "rules")
	_ = os.MkdirAll(rulesDir, 0o755)
	ruleFile := filepath.Join(rulesDir, "editing.md")
	content := []byte("# rule\n")
	AppendMemoryAudit(lay, ruleFile, "write_behavior_policy", content)
	raw, err := os.ReadFile(filepath.Join(cwd, DotDir, "audit", "memory-write.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var rec auditRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("json: %v", err)
	}
	if rec.Path != ruleFile || rec.Source != "write_behavior_policy" {
		t.Fatalf("record: %+v", rec)
	}
}

func TestPathUnderRoot(t *testing.T) {
	root := filepath.Join("/tmp", "r")
	if !pathUnderRoot(filepath.Join(root, "a", "b"), root) {
		t.Fatal("expected under")
	}
	if pathUnderRoot(filepath.Join(root, "..", "etc"), root) {
		t.Fatal("expected not under")
	}
}

func TestAppendMaintenanceSection_Audits(t *testing.T) {
	t.Setenv("ONCLAW_DISABLE_MEMORY_AUDIT", "")
	cwd := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	lay := DefaultLayout(cwd, home)
	memPath := filepath.Join(lay.Project, entrypointName)
	section := "## Auto-maintained (2099-01-01)\n- test bullet\n"
	if err := appendMaintenanceSection(lay, memPath, section, AuditSourcePostTurnMaintain); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(cwd, DotDir, "audit", "memory-write.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(raw))
	var rec auditRecord
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Source != AuditSourcePostTurnMaintain {
		t.Fatalf("got %q", rec.Source)
	}
}
