package wfexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/engine"
)

func TestBuildSubagentUserPrompt_runJournalPathMetadata(t *testing.T) {
	tmp := t.TempDir()
	host := "default"
	journalDir := filepath.Join(tmp, "runs", host)
	if err := os.MkdirAll(journalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jpath := filepath.Join(journalDir, "runs.jsonl")
	payload := []byte(`{"phase":"run_start","detail":{"correlation_id":"c1"}}` + "\n")
	if err := os.WriteFile(jpath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{
		SessionRoot: tmp,
		Turn:        engine.TurnContext{AgentID: host},
		Agent:       nil,
		CorrelationID: "c1",
		CurrentParams: map[string]any{
			"context": []any{
				map[string]any{
					"ref": "run_journal", "scope": "current_turn",
					"as": "path_metadata", "label": "Run record",
				},
			},
		},
	}
	s, err := BuildSubagentUserPrompt(rtx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, jpath) || !strings.Contains(s, "size_bytes:") {
		t.Fatalf("unexpected prompt:\n%s", s)
	}
}

func TestBuildSubagentUserPrompt_defaultTurnPair(t *testing.T) {
	rtx := &engine.RuntimeContext{
		UserPrompt:    "hi",
		Assistant:     "hello",
		CurrentParams: map[string]any{},
	}
	s, err := BuildSubagentUserPrompt(rtx)
	if err != nil {
		t.Fatal(err)
	}
	if s == "" || !strings.Contains(s, "hi") || !strings.Contains(s, "hello") {
		t.Fatalf("unexpected prompt: %q", s)
	}
}
