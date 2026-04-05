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
