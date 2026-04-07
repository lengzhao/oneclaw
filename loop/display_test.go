package loop_test

import (
	"testing"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/openai/openai-go"
)

func TestLastAssistantDisplay(t *testing.T) {
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("hi"),
		openai.AssistantMessage("Hello!"),
	}
	if got := loop.LastAssistantDisplay(msgs); got != "Hello!" {
		t.Fatalf("got %q", got)
	}
}

func TestAssistantParamText_plain(t *testing.T) {
	m := openai.AssistantMessage("  hi  ")
	if got := loop.AssistantParamText(m); got != "hi" {
		t.Fatalf("got %q", got)
	}
}
