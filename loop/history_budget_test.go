package loop

import (
	"strings"
	"testing"

	"github.com/openai/openai-go"
)

func TestTrimMessagesToBudget_dropsOldestUser(t *testing.T) {
	var msgs []openai.ChatCompletionMessageParamUnion
	for i := 0; i < 20; i++ {
		msgs = append(msgs, openai.UserMessage(strings.Repeat("u", 500)))
	}
	trimmed := TrimMessagesToBudget(msgs, 4000, 4)
	if len(trimmed) >= len(msgs) {
		t.Fatalf("expected fewer messages")
	}
	if len(trimmed) < 4 {
		t.Fatalf("expected min tail preserved, got %d", len(trimmed))
	}
}

func TestDropOldestPrefix_twoUsers(t *testing.T) {
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("a"),
		openai.UserMessage("b"),
	}
	next, ok := dropOldestPrefix(msgs)
	if !ok || len(next) != 1 {
		t.Fatalf("got ok=%v len=%d", ok, len(next))
	}
}
