//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

type auditLine struct {
	TS     string `json:"ts"`
	Source string `json:"source"`
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// E2E-93 PostTurn 追加 daily log 后写入审计 JSONL（source=daily_log_line）
func TestE2E_93_MemoryAuditDailyLog(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "assistant line for audit e2e"))
	e2eEnvWithMemory(t, stub)
	s := rtopts.Current()
	s.DisableMemoryExtract = false
	s.DisableMemoryAudit = false
	rtopts.Set(&s)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "user line for audit e2e"}); err != nil {
		t.Fatal(err)
	}
	lay := memory.DefaultLayout(cwd, home)
	logPath := memory.DailyLogPath(lay.Auto, time.Now().Format("2006-01-02"))
	auditPath := filepath.Join(cwd, memory.DotDir, "audit", "memory-write.jsonl")
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit file: %v", err)
	}
	line := strings.TrimSpace(string(raw))
	var rec auditLine
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("audit json: %v\n%s", err, line)
	}
	if rec.Source != "daily_log_line" {
		t.Fatalf("source=%q want daily_log_line", rec.Source)
	}
	if rec.Path != logPath {
		t.Fatalf("path=%q want %q", rec.Path, logPath)
	}
	if rec.Bytes < 10 || len(rec.SHA256) != 64 {
		t.Fatalf("record: %+v", rec)
	}
}

// E2E-94 features.disable_memory_audit 时不落盘审计文件
func TestE2E_94_MemoryAuditDisabledNoFile(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "x"))
	e2eEnvWithMemory(t, stub)
	s := rtopts.Current()
	s.DisableMemoryExtract = false
	s.DisableMemoryAudit = true
	rtopts.Set(&s)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "y"}); err != nil {
		t.Fatal(err)
	}
	auditPath := filepath.Join(cwd, memory.DotDir, "audit", "memory-write.jsonl")
	if _, err := os.Stat(auditPath); err == nil {
		t.Fatalf("audit file should not exist: %s", auditPath)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// E2E-95 write_file 写入 memory 根时追加审计（source=write_file）
func TestE2E_95_MemoryAuditWriteFileUnderMemoryRoot(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	stub := openaistub.New(t)
	memRel := filepath.Join(".oneclaw", "memory", "audit_write_e2e.md")
	content := "E2E95_AUDIT_WRITE_MARKER\n"
	args, err := json.Marshal(map[string]string{"path": memRel, "content": content})
	if err != nil {
		t.Fatal(err)
	}
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("w", "write_file", string(args)),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvWithMemory(t, stub)
	s95 := rtopts.Current()
	s95.DisableMemoryAudit = false
	rtopts.Set(&s95)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "write memory topic"}); err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(cwd, memRel)
	auditPath := filepath.Join(cwd, memory.DotDir, "audit", "memory-write.jsonl")
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit file: %v", err)
	}
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec auditLine
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		if rec.Source == "write_file" && rec.Path == wantPath {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no write_file audit for %q in:\n%s", wantPath, string(raw))
	}
}
