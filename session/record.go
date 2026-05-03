// Package session appends transcript and per-agent run records (FR-AGT-05 baseline).
package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// TranscriptTurn is one JSON line in sessions/<id>/transcript.jsonl.
type TranscriptTurn struct {
	Ts      time.Time `json:"ts"`
	Role    string    `json:"role"`
	Content string    `json:"content"`
}

// AppendTranscriptTurn appends one transcript record (creates parent dirs).
func AppendTranscriptTurn(sessionRoot string, t TranscriptTurn) error {
	path := filepath.Join(sessionRoot, "transcript.jsonl")
	return appendJSONL(path, t)
}

// RunEvent is one JSON line under sessions/<id>/runs/<agent_type>/runs.jsonl.
type RunEvent struct {
	Ts        time.Time      `json:"ts"`
	AgentType string         `json:"agent_type"`
	Phase     string         `json:"phase"`
	Detail    map[string]any `json:"detail,omitempty"`
}

// AppendRunEvent appends an execution record for an agent_type.
func AppendRunEvent(sessionRoot, agentType string, e RunEvent) error {
	dir := filepath.Join(sessionRoot, "runs", agentType)
	path := filepath.Join(dir, "runs.jsonl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return appendJSONL(path, e)
}

func appendJSONL(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
