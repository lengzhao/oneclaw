package loop

import (
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/budget"
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

func TestApplyHistoryBudget_insertsCompactBoundary(t *testing.T) {
	var msgs []openai.ChatCompletionMessageParamUnion
	for range 200 {
		msgs = append(msgs, openai.UserMessage(strings.Repeat("z", 900)))
	}
	g := budget.Global{MaxPromptBytes: 120_000, MinTailMessages: 6}
	before := len(msgs)
	ApplyHistoryBudget(g, "system-prompt", &msgs)
	if len(msgs) >= before {
		t.Fatalf("expected trim/compact, before=%d after=%d", before, len(msgs))
	}
	u := UserMessageText(msgs[0])
	if !strings.Contains(u, "compact_boundary") {
		previewLen := 200
		if len(u) < previewLen {
			previewLen = len(u)
		}
		t.Fatalf("expected compact_boundary in first message, got %q", u[:previewLen])
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
