package wfexec

import (
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/session"
)

func TestAdkMessagesForMain_withoutLoadTranscriptUsesPromptOnly(t *testing.T) {
	rtx := &engine.RuntimeContext{UserPrompt: "hello"}
	msgs, err := adkMessagesForMain(rtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("want 1 message, got %d", len(msgs))
	}
	m := msgs[0]
	if m.Role != schema.User || m.Content != "hello" {
		t.Fatalf("want single user hello, got role=%q content=%q", m.Role, m.Content)
	}
}

func TestAdkMessagesForMain_withLoadTranscriptReplay(t *testing.T) {
	rtx := &engine.RuntimeContext{
		UserPrompt: "current-turn",
		TranscriptReplayTurns: []session.TranscriptTurn{
			{Ts: time.Now(), Role: "user", Content: "u1"},
			{Ts: time.Now(), Role: "assistant", Content: "a1"},
		},
	}
	msgs, err := adkMessagesForMain(rtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("want history u1,a1 + current user = 3 messages, got %d", len(msgs))
	}
	last := msgs[2]
	if last.Role != schema.User || last.Content != "current-turn" {
		t.Fatalf("want final user current-turn, got role=%q content=%q", last.Role, last.Content)
	}
}

func TestAdkMessagesForMain_recallBetweenHistoryAndCurrent(t *testing.T) {
	rtx := &engine.RuntimeContext{
		UserPrompt: "fix-it",
		PromptTemplateData: map[string]any{
			"MemoryRecall": "## Memory recall (instruction root)\n\n- note from memory/",
		},
		TranscriptReplayTurns: []session.TranscriptTurn{
			{Ts: time.Now(), Role: "user", Content: "prior"},
		},
	}
	msgs, err := adkMessagesForMain(rtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("want history + recall + current = 3, got %d", len(msgs))
	}
	if msgs[0].Content != "prior" || msgs[2].Content != "fix-it" {
		t.Fatalf("unexpected ordering: %#v", msgs)
	}
	if !strings.Contains(msgs[1].Content, "Memory recall") || !strings.Contains(msgs[1].Content, "note from memory") {
		t.Fatalf("want recall section in middle message, got %q", msgs[1].Content)
	}
}
