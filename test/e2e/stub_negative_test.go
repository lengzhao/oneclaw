//go:build e2e

package e2e_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

// E2E-81 空用户输入被拒绝
func TestE2E_81_EmptyInboundRejected(t *testing.T) {
	stub := openaistub.New(t)
	e2eEnvMinimal(t, stub)
	e := newStubEngine(t, stub, t.TempDir())
	err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "   "})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err=%v", err)
	}
}

// E2E-82 未注册工具名：返回 unknown tool，随后 assistant 仍可结束
func TestE2E_82_UnknownToolName(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("c1", "nonexistent_tool_xyz", `{}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "done"))
	e2eEnvMinimal(t, stub)

	cwd := t.TempDir()
	client := openai.NewClient(stubOpenAIOptions(stub)...)
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err := loop.RunTurn(context.Background(), loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "test",
		MaxTokens:   256,
		MaxSteps:    8,
		Messages:    &msgs,
		Registry:    tools.NewRegistry(),
		ToolContext: toolctx.New(cwd, context.Background()),
	}, bus.InboundMessage{Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	var toolBody string
	for _, m := range msgs {
		if m.OfTool != nil && m.OfTool.Content.OfString.Valid() {
			toolBody = m.OfTool.Content.OfString.Value
			break
		}
	}
	if !strings.Contains(toolBody, "unknown tool") {
		t.Fatalf("tool body %q", toolBody)
	}
	last := msgs[len(msgs)-1]
	if last.OfAssistant == nil || !last.OfAssistant.Content.OfString.Valid() ||
		last.OfAssistant.Content.OfString.Value != "done" {
		t.Fatalf("last=%#v", last)
	}
}
