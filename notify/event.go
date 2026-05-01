package notify

import (
	"strings"
	"time"
	"unicode/utf8"
)

// SchemaVersion for JSON consumers of notify.Event (hook envelope).
const SchemaVersion = 3

// MVP lifecycle events delivered to [Sink] implementations (lightweight).
// Detailed model steps, tools, and nested runs are written to per-turn execution shards by session; see session/exec_journal.go.
const (
	EventUserInput = "user_input"
	EventTurnStart = "turn_start"
	EventTurnEnd   = "turn_end"
)

// Default preview length for user-visible and tool summaries in payloads.
const DefaultPreviewRunes = 512

// Event is one notify record: correlation on the envelope, type-specific fields in Data.
type Event struct {
	SchemaVersion int    `json:"schema_version"`
	Event         string `json:"event"`
	// TS is Unix time in milliseconds (UTC).
	TS            int64          `json:"ts"`
	Severity      string         `json:"severity,omitempty"`
	SessionID     string         `json:"session_id,omitempty"`
	AgentID       string         `json:"agent_id,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	TurnID        string         `json:"turn_id,omitempty"`
	ParentAgentID string         `json:"parent_agent_id,omitempty"`
	RunID         string         `json:"run_id,omitempty"`
	ParentRunID   string         `json:"parent_run_id,omitempty"`
	Data          map[string]any `json:"data,omitempty"`
}

// NewEvent fills schema_version, ts (Unix ms), event, and optional severity.
func NewEvent(kind, severity string) Event {
	ev := Event{
		SchemaVersion: SchemaVersion,
		Event:         kind,
		TS:            time.Now().UnixMilli(),
		Data:          map[string]any{},
	}
	if severity != "" {
		ev.Severity = severity
	}
	return ev
}

// Preview returns a UTF-8 safe prefix of s for logs and hooks.
func Preview(s string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = DefaultPreviewRunes
	}
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", " "), "\t", " "))
	if s == "" || !utf8.ValidString(s) {
		return ""
	}
	var n int
	for i := range s {
		if n == maxRunes {
			return s[:i] + "…"
		}
		n++
	}
	return s
}
