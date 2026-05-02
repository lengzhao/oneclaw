//go:build e2e

package e2e_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-02 同 session 多轮：共享 Messages，连续两次 SubmitUser。
func TestE2E_02_MultiTurnSameSession(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "first reply"))
	stub.Enqueue(openaistub.CompletionStop("", "second reply"))
	e2eEnvMinimal(t, stub)

	cwd := t.TempDir()
	e := newStubEngine(t, stub, cwd)

	if err := e.SubmitUser(context.Background(), stubInbound("turn one")); err != nil {
		t.Fatal(err)
	}
	nAfterFirst := len(e.Messages)
	if nAfterFirst < 2 {
		t.Fatalf("after turn1 expected >=2 msgs, got %d", nAfterFirst)
	}
	if got := loop.LastAssistantDisplay(e.Messages); got != "first reply" {
		t.Fatalf("turn1 last assistant %q", got)
	}

	if err := e.SubmitUser(context.Background(), stubInbound("turn two")); err != nil {
		t.Fatal(err)
	}
	if len(e.Messages) <= nAfterFirst {
		t.Fatalf("after turn2 expected more msgs than %d, got %d", nAfterFirst, len(e.Messages))
	}
	if got := loop.LastAssistantDisplay(e.Messages); got != "second reply" {
		t.Fatalf("turn2 last assistant %q", got)
	}
}

// E2E-04 写后读：write_file → read_file → 结束文本。
func TestE2E_04_WriteThenRead(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("call_w", "write_file", `{"path":"subdir/x.txt","content":"hello e2e"}`),
	}))
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("call_r", "read_file", `{"path":"subdir/x.txt"}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "verified"))
	e2eEnvMinimal(t, stub)

	cwd := t.TempDir()
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), stubInbound("create and read subdir/x.txt")); err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(cwd, "subdir", "x.txt")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("file missing: %v", err)
	}
	if string(b) != "hello e2e" {
		t.Fatalf("file content %q", b)
	}

	if got := loop.LastAssistantDisplay(e.Messages); got != "verified" {
		t.Fatalf("final assistant %q", got)
	}
}

// E2E-05 Abort：在 SubmitUser 开始前取消 context，应返回 context.Canceled。
func TestE2E_05_AbortCanceledContext(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "should not run"))
	e2eEnvMinimal(t, stub)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	e := newStubEngine(t, stub, t.TempDir())
	err := e.SubmitUser(ctx, stubInbound("hi"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
