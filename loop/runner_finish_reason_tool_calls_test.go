package loop

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// Regression: empty finish_reason with tool_calls must still run tools and reach a final assistant hop.
func TestRunTurn_toolCallsWhenFinishReasonEmpty(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCallsEmptyFinishReason("", []map[string]any{
		openaistub.ToolCall("c1", "forbidden", `{}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "done after tool"))

	t.Cleanup(func() { rtopts.Set(nil) })
	snap := rtopts.DefaultSnapshot()
	snap.ChatTransport = "non_stream"
	rtopts.Set(&snap)

	var execCount atomic.Int32
	reg := tools.NewRegistry()
	if err := reg.Register(countingDenyTool{execs: &execCount}); err != nil {
		t.Fatal(err)
	}

	client := openai.NewClient(
		option.WithAPIKey("sk-test-stub"),
		option.WithBaseURL(stub.BaseURL()),
	)
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err := RunTurn(context.Background(), Config{
		Client:        &client,
		Model:         "gpt-4o",
		System:        "test",
		MaxTokens:     256,
		MaxSteps:      2,
		Messages:      &msgs,
		Registry:      reg,
		ToolContext:   toolctx.New(t.TempDir(), context.Background()),
		ChatTransport: "non_stream",
		TurnMaxSteps:  2,
	}, bus.InboundMessage{Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if execCount.Load() != 1 {
		t.Fatalf("tool should run when finish_reason is empty but tool_calls present, execCount=%d", execCount.Load())
	}
	last := msgs[len(msgs)-1]
	if last.OfAssistant == nil || !last.OfAssistant.Content.OfString.Valid() ||
		last.OfAssistant.Content.OfString.Value != "done after tool" {
		t.Fatalf("final assistant: %#v", last.OfAssistant)
	}
	if len(stub.ChatRequestBodies()) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(stub.ChatRequestBodies()))
	}
}
