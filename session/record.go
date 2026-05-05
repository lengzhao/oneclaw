// Package session appends transcript and per-agent run records (FR-AGT-05 baseline).
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TranscriptTurn is one JSON line in sessions/<id>/{agent_type}_transcript.jsonl.
type TranscriptTurn struct {
	Ts      time.Time `json:"ts"`
	Role    string    `json:"role"`
	Content string    `json:"content"`
}

// AppendTranscriptTurn appends one transcript record (creates parent dirs).
// agentType is usually the catalog agent id (for example: default, memory_extractor, skill_generator).
func AppendTranscriptTurn(sessionRoot, agentType string, t TranscriptTurn) error {
	path := transcriptPath(sessionRoot, agentType)
	return appendJSONL(path, t)
}

// RunEvent is one JSON line under sessions/<id>/runs/<agent_type>/<journal_key>.jsonl (one file per turn / sub-agent run).
type RunEvent struct {
	Ts        time.Time      `json:"ts"`
	AgentType string         `json:"agent_type"`
	Phase     string         `json:"phase"`
	Detail    map[string]any `json:"detail,omitempty"`
}

// TurnRunJournalPath returns sessions/<id>/runs/<agent_type>/<journal_key>.jsonl (one file per host turn or sub-agent run).
func TurnRunJournalPath(sessionRoot, agentType, journalKey string) string {
	key := strings.TrimSpace(journalKey)
	at := strings.TrimSpace(agentType)
	return filepath.Join(strings.TrimSpace(sessionRoot), "runs", at, key+".jsonl")
}

// AppendTurnRunEvent appends one line to the per-turn journal file (runs/<agent>/<journal_key>.jsonl).
func AppendTurnRunEvent(sessionRoot, agentType, journalKey string, e RunEvent) error {
	key := strings.TrimSpace(journalKey)
	if key == "" {
		return fmt.Errorf("turn journal key required")
	}
	dir := filepath.Join(strings.TrimSpace(sessionRoot), "runs", strings.TrimSpace(agentType))
	path := filepath.Join(dir, key+".jsonl")
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
