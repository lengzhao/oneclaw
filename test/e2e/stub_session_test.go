//go:build e2e

package e2e_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/openai/openai-go"
)

// E2E-02 同 session 多轮：共享 Messages，连续两次 RunTurn。
func TestE2E_02_MultiTurnSameSession(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "first reply"))
	stub.Enqueue(openaistub.CompletionStop("", "second reply"))
	e2eEnvMinimal(t, stub)

	cwd := t.TempDir()
	client := openai.NewClient(stubOpenAIOptions(stub)...)
	msgs := []openai.ChatCompletionMessageParamUnion{}
	cfg := loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "test",
		MaxTokens:   256,
		MaxSteps:    4,
		Messages:    &msgs,
		Registry:    builtin.DefaultRegistry(),
		ToolContext: toolctx.New(cwd, context.Background()),
	}

	if err := loop.RunTurn(context.Background(), cfg, bus.InboundMessage{Content: "turn one"}); err != nil {
		t.Fatal(err)
	}
	nAfterFirst := len(msgs)
	if nAfterFirst < 2 {
		t.Fatalf("after turn1 expected >=2 msgs, got %d", nAfterFirst)
	}
	last := msgs[len(msgs)-1]
	if last.OfAssistant == nil || !last.OfAssistant.Content.OfString.Valid() ||
		last.OfAssistant.Content.OfString.Value != "first reply" {
		t.Fatalf("turn1 last msg: %#v", last)
	}

	if err := loop.RunTurn(context.Background(), cfg, bus.InboundMessage{Content: "turn two"}); err != nil {
		t.Fatal(err)
	}
	if len(msgs) <= nAfterFirst {
		t.Fatalf("after turn2 expected more msgs than %d, got %d", nAfterFirst, len(msgs))
	}
	last = msgs[len(msgs)-1]
	if last.OfAssistant == nil || !last.OfAssistant.Content.OfString.Valid() ||
		last.OfAssistant.Content.OfString.Value != "second reply" {
		t.Fatalf("turn2 last msg: %#v", last)
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
	client := openai.NewClient(stubOpenAIOptions(stub)...)
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err := loop.RunTurn(context.Background(), loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "Use tools.",
		MaxTokens:   256,
		MaxSteps:    12,
		Messages:    &msgs,
		Registry:    builtin.DefaultRegistry(),
		ToolContext: toolctx.New(cwd, context.Background()),
	}, bus.InboundMessage{Content: "create and read subdir/x.txt"})
	if err != nil {
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

	last := msgs[len(msgs)-1]
	if last.OfAssistant == nil || !last.OfAssistant.Content.OfString.Valid() ||
		last.OfAssistant.Content.OfString.Value != "verified" {
		t.Fatalf("final assistant: %#v", last)
	}
}

// E2E-05 Abort：在 RunTurn 开始前取消 context，应返回 context.Canceled。
func TestE2E_05_AbortCanceledContext(t *testing.T) {
	stub := openaistub.New(t)
	// If RunTurn ever reached the server, this would be consumed; canceled path should not.
	stub.Enqueue(openaistub.CompletionStop("", "should not run"))
	e2eEnvMinimal(t, stub)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cwd := t.TempDir()
	client := openai.NewClient(stubOpenAIOptions(stub)...)
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err := loop.RunTurn(ctx, loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "test",
		MaxTokens:   256,
		MaxSteps:    4,
		Messages:    &msgs,
		Registry:    builtin.DefaultRegistry(),
		ToolContext: toolctx.New(cwd, context.Background()),
	}, bus.InboundMessage{Content: "hi"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
