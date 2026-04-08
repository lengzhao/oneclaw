package loop

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go"
)

func TestRequestMessagesJSONArray(t *testing.T) {
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("sys"),
		openai.UserMessage("hi"),
	}
	b, err := RequestMessagesJSONArray(msgs)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []json.RawMessage
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 {
		t.Fatalf("len=%d", len(decoded))
	}
}
