package loop

import (
	"strings"

	"github.com/openai/openai-go"
)

func assistantVisibleText(msg openai.ChatCompletionMessage) string {
	if msg.Refusal != "" {
		return "[refusal] " + msg.Refusal
	}
	return strings.TrimSpace(msg.Content)
}
