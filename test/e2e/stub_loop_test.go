// 会话与工具闭环的 stub E2E；用例编号见 CASES.md。
package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/openai/openai-go"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-01 最小对话（纯文本）
func TestE2E_StubTextReply(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "hello from stub"))
	e2eEnvMinimal(t, stub)

	cwd := t.TempDir()
	client := openai.NewClient()
	msgs := []openai.ChatCompletionMessageParamUnion{}
	reg := tools.NewRegistry()
	err := loop.RunTurn(context.Background(), loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "You are a test assistant.",
		MaxTokens:   256,
		MaxSteps:    4,
		Messages:    &msgs,
		Registry:    reg,
		ToolContext: toolctx.New(cwd, context.Background()),
	}, routing.Inbound{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) < 2 {
		t.Fatalf("expected user+assistant, got %d messages", len(msgs))
	}
	last := msgs[len(msgs)-1]
	if last.OfAssistant == nil {
		t.Fatalf("last message not assistant: %#v", last)
	}
	if !last.OfAssistant.Content.OfString.Valid() || last.OfAssistant.Content.OfString.Value != "hello from stub" {
		t.Fatalf("assistant content: %#v", last.OfAssistant.Content)
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

	client := openai.NewClient()
	msgs := []openai.ChatCompletionMessageParamUnion{}
	reg := builtin.DefaultRegistry()

	err := loop.RunTurn(context.Background(), loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "Use tools when needed.",
		MaxTokens:   256,
		MaxSteps:    8,
		Messages:    &msgs,
		Registry:    reg,
		ToolContext: toolctx.New(cwd, context.Background()),
	}, routing.Inbound{Text: "read note.txt"})
	if err != nil {
		t.Fatal(err)
	}

	var sawTool bool
	for _, m := range msgs {
		if m.OfTool != nil {
			sawTool = true
		}
	}
	if !sawTool {
		t.Fatalf("expected a tool message in transcript, got %d msgs", len(msgs))
	}
	last := msgs[len(msgs)-1]
	if last.OfAssistant == nil {
		t.Fatalf("expected final assistant, got %#v", last)
	}
	if !last.OfAssistant.Content.OfString.Valid() || last.OfAssistant.Content.OfString.Value != "read ok" {
		t.Fatalf("final assistant: %#v", last.OfAssistant.Content)
	}
}
