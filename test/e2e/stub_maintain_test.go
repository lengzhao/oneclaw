package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-92 模型化维护：回合结束后读 daily log，第二次 stub 请求写回 project MEMORY.md。
func TestE2E_92_AutoMaintenanceAppends(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "main turn ok"))
	date := time.Now().Format("2006-01-02")
	section := "## Auto-maintained (" + date + ")\n- E2E92_MAINTAIN_MARKER\n"
	stub.Enqueue(openaistub.CompletionStop("", section))
	e2eEnvWithMemory(t, stub)

	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ONCLAW_DISABLE_AUTO_MAINTENANCE", "0")
	t.Setenv("ONCLAW_MAINTENANCE_MODEL", "gpt-4o")
	t.Setenv("ONCLAW_MAINTENANCE_MIN_LOG_BYTES", "50")
	e2eIsolateUserMemory(t, home)

	e := newStubEngine(t, cwd)
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "hello recallkeyword"}); err != nil {
		t.Fatal(err)
	}

	memPath := filepath.Join(cwd, memory.DotDir, "memory", "MEMORY.md")
	raw, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	if !strings.Contains(string(raw), "E2E92_MAINTAIN_MARKER") {
		t.Fatalf("expected maintainer marker in:\n%s", string(raw))
	}
}
