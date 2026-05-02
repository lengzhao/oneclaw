//go:build e2e

// 会话与工具闭环的 stub E2E；用例编号见 CASES.md。
package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-01 最小对话（纯文本）
func TestE2E_StubTextReply(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "hello from stub"))
	e2eEnvMinimal(t, stub)

	cwd := t.TempDir()
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), stubInbound("hi")); err != nil {
		t.Fatal(err)
	}
	e2eWaitMinChatRequests(t, stub, 1, 2*time.Second)
	got := loop.LastAssistantDisplay(e.Messages)
	if got != "hello from stub" {
		t.Fatalf("assistant reply %q want %q", got, "hello from stub")
	}
}

// E2E-03 工具调用闭环（read_file）
func TestE2E_StubToolThenText(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("call_1", "read_file", `{"path":"note.txt"}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "read ok"))
	e2eEnvMinimal(t, stub)

	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "note.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), stubInbound("read note.txt")); err != nil {
		t.Fatal(err)
	}
	e2eWaitMinChatRequests(t, stub, 2, 3*time.Second)

	got := loop.LastAssistantDisplay(e.Messages)
	if got != "read ok" {
		t.Fatalf("final assistant %q want read ok", got)
	}
}
