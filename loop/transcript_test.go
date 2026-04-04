package loop

import (
	"testing"

	"github.com/openai/openai-go"
)

func TestMarshalTranscriptRoundTrip(t *testing.T) {
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("hi"),
		openai.AssistantMessage("hello"),
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
}
