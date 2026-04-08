package notify

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Schema version for JSON consumers (bump when envelope fields change).
const SchemaVersion = 2

// MVP lifecycle event kinds (see docs/notification-hooks-design.md).
const (
	EventInboundReceived = "inbound_received"
	EventAgentTurnStart  = "agent_turn_start"
	// EventMemoryTurnContext is emitted after memory.BuildTurn / ApplyTurnBudget, before the model loop.
	// data carries the recall and agent-md blocks actually injected into this turn (see docs).
	EventMemoryTurnContext = "memory_turn_context"
	EventModelStepStart  = "model_step_start"
	EventModelStepEnd    = "model_step_end"
	// EventTurnFirstModelRequest marks the first model API call of the current user turn (step 0),
	// carrying the full request messages JSON (system + history as sent to the API).
	EventTurnFirstModelRequest = "turn_first_model_request"
	EventToolCallStart   = "tool_call_start"
	EventToolCallEnd     = "tool_call_end"
	EventSubagentStart   = "subagent_start"
	EventSubagentEnd     = "subagent_end"
	EventTurnComplete    = "turn_complete"
	EventTurnError       = "turn_error"
)

// Default preview length for user-visible and tool summaries in payloads.
const DefaultPreviewRunes = 512

// Event is one notify record: correlation on the envelope, type-specific fields in Data.
type Event struct {
	SchemaVersion int            `json:"schema_version"`
	Event         string         `json:"event"`
	// TS is Unix time in milliseconds (UTC).
	TS int64 `json:"ts"`
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
