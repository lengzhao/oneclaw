//go:build e2e

// 子 Agent / fork：阶段 C 的 stub E2E。
package e2e_test

import (
	"context"
	"os"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/workspace"
)

// E2E-90 run_agent：主线程 transcript 与子循环隔离；子循环单独消耗 stub 队列项。
func TestE2E_StubRunAgentNested(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("c1", "run_agent", `{"agent_type":"explore","prompt":"scan repo"}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "sub finished"))
	stub.Enqueue(openaistub.CompletionStop("", "parent done"))
	e2eEnvMinimal(t, stub)

	cwd := t.TempDir()
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "go"}); err != nil {
		t.Fatal(err)
	}

	last := e.Messages[len(e.Messages)-1]
	if last.OfAssistant == nil || !last.OfAssistant.Content.OfString.Valid() ||
		last.OfAssistant.Content.OfString.Value != "parent done" {
		t.Fatalf("expected parent final assistant, got %#v", last)
	}

	sideDir := workspace.JoinSessionWorkspace(cwd, false, "sidechain")
	entries, err := os.ReadDir(sideDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected sidechain files under %s: %v entries=%v", sideDir, err, entries)
	}
}

// E2E-91 fork_context：共享主 system，子调用消耗独立队列项。
func TestE2E_StubForkContext(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("f1", "fork_context", `{"prompt":"summarize briefly"}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "fork answer"))
	stub.Enqueue(openaistub.CompletionStop("", "ack"))
	e2eEnvMinimal(t, stub)

	cwd := t.TempDir()
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	last := e.Messages[len(e.Messages)-1]
	if last.OfAssistant == nil || !last.OfAssistant.Content.OfString.Valid() ||
		last.OfAssistant.Content.OfString.Value != "ack" {
		t.Fatalf("expected final ack, got %#v", last)
	}
}
