package loop

import (
	"context"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

func TestRunTurnNilMessages(t *testing.T) {
	err := RunTurn(context.Background(), Config{
		Client:       &openai.Client{},
		Model:        "gpt-4o-mini",
		Messages:     nil,
		Registry:     tools.NewRegistry(),
		TurnMaxSteps: 8,
	}, bus.InboundMessage{Content: "hi"})
	if err == nil {
		t.Fatal("expected error for nil Messages")
	}
}

func TestRunTurnInvalidTurnMaxSteps(t *testing.T) {
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err := RunTurn(context.Background(), Config{
		Client:       &openai.Client{},
		Model:        "gpt-4o-mini",
		MaxSteps:     4,
		Messages:     &msgs,
		Registry:     tools.NewRegistry(),
		ToolContext:  toolctx.New(t.TempDir(), context.Background()),
		TurnMaxSteps: 0,
	}, bus.InboundMessage{Content: "hi"})
	if err == nil || !strings.Contains(err.Error(), "TurnMaxSteps") {
		t.Fatalf("expected TurnMaxSteps error, got %v", err)
	}
}
