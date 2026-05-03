package adkhost

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/lengzhao/oneclaw/tools"
)

func TestNewChatModelAgent_stub(t *testing.T) {
	ctx := context.Background()
	reg := tools.NewRegistry(t.TempDir())
	if err := tools.RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	ag, err := NewChatModelAgent(ctx, NewStubChatModel("hi"), reg, AgentOptions{Name: "t"})
	if err != nil {
		t.Fatal(err)
	}
	iter := ag.Run(ctx, &adk.AgentInput{
		Messages: []adk.Message{schema.UserMessage("yo")},
	})
	ev, ok := iter.Next()
	if !ok {
		t.Fatal("expected event")
	}
	if ev.Err != nil {
		t.Fatal(ev.Err)
	}
}
