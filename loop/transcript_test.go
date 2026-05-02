package loop

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestMarshalTranscriptRoundTrip(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("hi"),
		schema.AssistantMessage("hello", nil),
	}
	b, err := MarshalMessages(msgs)
	if err != nil {
		t.Fatal(err)
	}
	out, err := UnmarshalMessages(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("len %d", len(out))
	}
	if UserMessageText(out[0]) != "hi" {
		t.Fatalf("user: %q", UserMessageText(out[0]))
	}
	if AssistantParamText(out[1]) != "hello" {
		t.Fatalf("assistant: %q", AssistantParamText(out[1]))
	}
}
