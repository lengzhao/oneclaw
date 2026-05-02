package loop

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestVisibleTranscriptAppendSince(t *testing.T) {
	var tr []*schema.Message
	tr = append(tr, schema.UserMessage("u1"))
	tr = append(tr, schema.AssistantMessage("a1", nil))
	n0 := len(ToUserVisibleMessages(tr))

	tr = append(tr, schema.UserMessage("u2"))
	tr = append(tr, schema.AssistantMessage("a2", nil))
	delta := VisibleTranscriptAppendSince(tr, n0)
	if len(delta) != 2 || delta[0]["role"] != "user" || delta[0]["content"] != "u2" {
		t.Fatalf("delta user: %#v", delta)
	}
	if delta[1]["role"] != "assistant" || delta[1]["content"] != "a2" {
		t.Fatalf("delta assistant: %#v", delta)
	}
}
