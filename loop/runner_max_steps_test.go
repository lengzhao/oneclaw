package loop

import (
	"context"
	"encoding/json"
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

type countingDenyTool struct {
	execs *atomic.Int32
}

func (d countingDenyTool) Name() string             { return "forbidden" }
func (d countingDenyTool) ConcurrencySafe() bool    { return true }
func (d countingDenyTool) Description() string      { return "max-steps test tool" }
func (d countingDenyTool) Parameters() openai.FunctionParameters {
	return openai.FunctionParameters{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (d countingDenyTool) Execute(context.Context, json.RawMessage, *toolctx.Context) (string, error) {
	d.execs.Add(1)
	return "ran", nil
}

// Last model hop omits tools: prior step may still call tools; final step cannot.
func TestRunTurnMaxStepsFinalHopOmitsTools(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("c1", "forbidden", `{}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "summary only"))

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
	}, bus.InboundMessage{Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if execCount.Load() != 1 {
		t.Fatalf("tool should run on first hop only, execCount=%d", execCount.Load())
	}
	last := msgs[len(msgs)-1]
	if last.OfAssistant == nil || !last.OfAssistant.Content.OfString.Valid() ||
		last.OfAssistant.Content.OfString.Value != "summary only" {
		t.Fatalf("final assistant: %#v", last.OfAssistant)
	}

	bodies := stub.ChatRequestBodies()
	if len(bodies) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(bodies))
	}
	var req1, req2 struct {
		Tools json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(bodies[0], &req1); err != nil {
		t.Fatal(err)
	}
	if len(req1.Tools) == 0 || string(req1.Tools) == "null" || string(req1.Tools) == "[]" {
		t.Fatalf("first request should offer tools, got tools=%s", req1.Tools)
	}
	if err := json.Unmarshal(bodies[1], &req2); err != nil {
		t.Fatal(err)
	}
	if len(req2.Tools) > 0 && string(req2.Tools) != "null" && string(req2.Tools) != "[]" {
		t.Fatalf("last request should omit tools, got tools=%s", req2.Tools)
	}
}
