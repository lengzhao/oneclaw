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
		WorkflowExec: engine.WorkflowExec{
			CurrentParams: map[string]any{
				"context": []any{
					map[string]any{
						"ref": "run_journal", "scope": "current_turn",
						"as": "path_metadata", "label": "Run record",
					},
				},
			},
		},
		TurnInputs: engine.TurnInputs{
			SessionRoot:   tmp,
			Turn:          engine.TurnContext{AgentID: host},
			CorrelationID: "c1",
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
		WorkflowExec: engine.WorkflowExec{CurrentParams: map[string]any{}},
		TurnInputs:   engine.TurnInputs{UserPrompt: "hi"},
		ADKRuntime:   engine.ADKRuntime{Assistant: "hello"},
	}
	s, err := BuildSubagentUserPrompt(rtx)
	if err != nil {
		t.Fatal(err)
	}
	if s == "" || !strings.Contains(s, "hi") || !strings.Contains(s, "hello") {
		t.Fatalf("unexpected prompt: %q", s)
	}
}
