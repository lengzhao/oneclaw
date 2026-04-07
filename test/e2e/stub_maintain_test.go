package e2e_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-92 模型化维护：回合结束后近场维护（Current turn 快照），第二次 stub 请求写回 project `.oneclaw/memory/YYYY-MM-DD.md`。
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
	t.Setenv("ONCLAW_POST_TURN_MAINTENANCE_MIN_LOG_BYTES", "50")
	e2eIsolateUserMemory(t, home)

	e := newStubEngine(t, cwd)
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "hello recallkeyword"}); err != nil {
		t.Fatal(err)
	}

	epPath := memory.ProjectEpisodeDailyPath(cwd, date)
	raw, err := os.ReadFile(epPath)
	if err != nil {
		t.Fatalf("read episodic digest: %v", err)
	}
	if !strings.Contains(string(raw), "E2E92_MAINTAIN_MARKER") {
		t.Fatalf("expected maintainer marker in:\n%s", string(raw))
	}
}
