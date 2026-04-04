package session

import (
	"testing"

	"github.com/openai/openai-go"
)

func TestLastAssistantDisplay(t *testing.T) {
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("hi"),
		openai.AssistantMessage("Hello!"),
	}
	if got := LastAssistantDisplay(msgs); got != "Hello!" {
		t.Fatalf("got %q", got)
	}
}
