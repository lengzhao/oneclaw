package loop

import (
	"context"
	"testing"

	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
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
