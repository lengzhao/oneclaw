//go:build e2e

package e2e_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-92 回合后结构化记忆：第二次 stub 请求为 JSON extract，写入 `<auto>/agent_memory.sqlite`。
func TestE2E_92_AutoMaintenanceAppends(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "main turn ok"))
	extractJSON := `{"memories":[{"namespace":"knowledge","title":"e2e92","content":"E2E92_MAINTAIN_MARKER","summary":"","tags":[],"importance":50,"confidence":0.9,"reasoning":"e2e"}]}`
	stub.Enqueue(openaistub.CompletionStop("", extractJSON))
	e2eEnvWithMemory(t, stub)

	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	s := rtopts.Current()
	s.DisableAutoMaintenance = false
	s.MaintenanceModel = "gpt-4o"
	s.MaintenanceMinLogBytes = 50
	s.PostTurnMinLogBytes = 50
	rtopts.Set(&s)
	e2eIsolateUserMemory(t, home)

	e := newStubEngine(t, stub, cwd)
	stubAttachPostTurnExtractLLM(e, stub)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "hello recallkeyword"}); err != nil {
		t.Fatal(err)
	}
	e2eWaitMinChatRequests(t, stub, 2, 5*time.Second)

	lay := memory.DefaultLayout(cwd, home)
	sqlitePath := filepath.Join(lay.Auto, "agent_memory.sqlite")
	e2eWaitForFile(t, sqlitePath, 5*time.Second)
	e2eWaitAgentMemorySubstring(t, sqlitePath, "E2E92_MAINTAIN_MARKER", 3*time.Second)
}
