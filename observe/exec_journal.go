package observe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ExecRecord is one JSONL line for agent/tool execution (FR-OBS / FR-AGT-05 oriented).
type ExecRecord struct {
	Time      time.Time      `json:"ts"`
	SessionID string         `json:"session_id,omitempty"`
	AgentType string         `json:"agent_type,omitempty"`
	Phase     string         `json:"phase,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
}

// AppendExecJournal appends one JSON object as a line to path (creates parent dirs).
// CLI `run` logs lifecycle under sessions/<id>/runs/<agent>/runs.jsonl instead; use this for finer-grained execution traces when needed.
func AppendExecJournal(path string, rec ExecRecord) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(rec)
}
