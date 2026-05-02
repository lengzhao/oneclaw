package loop_test

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/oneclaw/loop"
)

func TestLastAssistantDisplay(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("hi"),
		schema.AssistantMessage("Hello!", nil),
	}
	if got := loop.LastAssistantDisplay(msgs); got != "Hello!" {
		t.Fatalf("got %q", got)
	}
}

func TestAssistantParamText_plain(t *testing.T) {
	m := schema.AssistantMessage("  hi  ", nil)
	if got := loop.AssistantParamText(m); got != "hi" {
		t.Fatalf("got %q", got)
	}
}
