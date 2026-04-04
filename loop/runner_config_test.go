package loop

import (
	"context"
	"testing"

	"github.com/openai/openai-go"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/tools"
)

func TestRunTurnNilMessages(t *testing.T) {
	err := RunTurn(context.Background(), Config{
		Client:   &openai.Client{},
		Model:    "gpt-4o-mini",
		Messages: nil,
		Registry: tools.NewRegistry(),
	}, routing.Inbound{Text: "hi"})
	if err == nil {
		t.Fatal("expected error for nil Messages")
	}
}
