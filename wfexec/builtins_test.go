package wfexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/session"
)

func TestAdkMessagesForMain_withoutLoadTranscriptUsesPromptOnly(t *testing.T) {
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "hello"}}
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
		TurnInputs: engine.TurnInputs{UserPrompt: "current-turn"},
		PromptScratch: engine.PromptScratch{
			TranscriptReplayTurns: []session.TranscriptTurn{
				{Ts: time.Now(), Role: "user", Content: "u1"},
				{Ts: time.Now(), Role: "assistant", Content: "a1"},
			},
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
		TurnInputs: engine.TurnInputs{UserPrompt: "fix-it"},
		PromptScratch: engine.PromptScratch{
			PromptTemplateData: map[string]any{
				"MemoryRecall": "## Memory recall (instruction root)\n\n- note from memory/",
			},
			TranscriptReplayTurns: []session.TranscriptTurn{
				{Ts: time.Now(), Role: "user", Content: "prior"},
			},
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

func TestAdkMessagesForMain_disableMemoryRecall(t *testing.T) {
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			UserPrompt: "fix-it",
			Agent: &catalog.Agent{
				ContextProfile: catalog.ContextProfile{Disable: []string{"memory_recall"}},
			},
		},
		PromptScratch: engine.PromptScratch{
			PromptTemplateData: map[string]any{
				"MemoryRecall": "## Memory recall (instruction root)\n\n- should not appear",
			},
		},
	}
	msgs, err := adkMessagesForMain(rtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("memory recall should be disabled, got %#v", msgs)
	}
}

func TestPrepareAgentContext_defaultFullContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "todo.json"), []byte(`{"items":[{"title":"ship"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	memDir := filepath.Join(dir, "memory", "2026-05")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "note.md"), []byte("durable fact"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := session.AppendTranscriptTurn(dir, "default", session.TranscriptTurn{Ts: time.Now(), Role: "user", Content: "prior"}); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			InstructionRoot: dir,
			SessionRoot:     dir,
			UserDataRoot:    dir,
			Agent:           &catalog.Agent{AgentType: "default"},
		},
		PromptScratch: engine.PromptScratch{PromptTemplateData: map[string]any{}},
	}

	if err := prepareAgentContext(rtx); err != nil {
		t.Fatal(err)
	}
	if got := rtx.PromptTemplateData["MemoryRecall"]; !strings.Contains(strings.TrimSpace(got.(string)), "memory/2026-05/note.md") {
		t.Fatalf("missing memory recall: %#v", got)
	}
	if got := rtx.PromptTemplateData["Tasks"]; !strings.Contains(strings.TrimSpace(got.(string)), "ship") {
		t.Fatalf("missing tasks: %#v", got)
	}
	if len(rtx.TranscriptReplayTurns) != 1 {
		t.Fatalf("expected transcript replay, got %#v", rtx.TranscriptReplayTurns)
	}
}

func TestPrepareAgentContext_disableContextBlocks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "todo.json"), []byte(`{"items":[{"title":"hidden"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	memDir := filepath.Join(dir, "memory", "2026-05")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "note.md"), []byte("hidden memory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := session.AppendTranscriptTurn(dir, "default", session.TranscriptTurn{Ts: time.Now(), Role: "user", Content: "hidden prior"}); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			InstructionRoot: dir,
			SessionRoot:     dir,
			UserDataRoot:    dir,
			Agent: &catalog.Agent{
				AgentType: "memory_extractor",
				ContextProfile: catalog.ContextProfile{
					Disable: []string{"memory_recall", "tasks", "transcript"},
				},
			},
		},
		PromptScratch: engine.PromptScratch{PromptTemplateData: map[string]any{}},
	}

	if err := prepareAgentContext(rtx); err != nil {
		t.Fatal(err)
	}
	if _, ok := rtx.PromptTemplateData["MemoryRecall"]; ok {
		t.Fatalf("memory recall should be disabled: %#v", rtx.PromptTemplateData)
	}
	if _, ok := rtx.PromptTemplateData["Tasks"]; ok {
		t.Fatalf("tasks should be disabled: %#v", rtx.PromptTemplateData)
	}
	if len(rtx.TranscriptReplayTurns) != 0 {
		t.Fatalf("transcript should be disabled: %#v", rtx.TranscriptReplayTurns)
	}
	if got := rtx.PromptTemplateData["SkillsIndex"]; strings.TrimSpace(got.(string)) == "" {
		t.Fatalf("skills should remain enabled by default: %#v", rtx.PromptTemplateData)
	}
}
