package loop

import (
	"testing"

	"github.com/openai/openai-go"
)

func TestVisibleTranscriptAppendSince(t *testing.T) {
	var tr []openai.ChatCompletionMessageParamUnion
	tr = append(tr, openai.UserMessage("u1"))
	tr = append(tr, openai.AssistantMessage("a1"))
	n0 := len(ToUserVisibleMessages(tr))

	tr = append(tr, openai.UserMessage("u2"))
	tr = append(tr, openai.AssistantMessage("a2"))
	delta := VisibleTranscriptAppendSince(tr, n0)
	if len(delta) != 2 || delta[0]["role"] != "user" || delta[0]["content"] != "u2" {
		t.Fatalf("delta user: %#v", delta)
	}
	if delta[1]["role"] != "assistant" || delta[1]["content"] != "a2" {
		t.Fatalf("delta assistant: %#v", delta)
	}
}
