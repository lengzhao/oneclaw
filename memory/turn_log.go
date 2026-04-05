package memory

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/loop"
)

const assistantTurnLogMaxRunes = 8000

// TurnLogPathForDate returns the append-only JSONL path for calendar day of `when` (UTC).
//
// Default (empty ONCLAW_TURN_LOG_PATH): <cwd>/.oneclaw/traces/logs/YYYY/MM/YYYY-MM-DD.jsonl
// — same hierarchy style as memory daily logs under logs/YYYY/MM/.
//
// ONCLAW_TURN_LOG_PATH:
//   - path ending in .jsonl → that single file (no date rotation);
//   - otherwise → directory root, files at <root>/logs/YYYY/MM/YYYY-MM-DD.jsonl
//     (relative paths are resolved under session cwd).
func TurnLogPathForDate(layout Layout, when time.Time) string {
	p := strings.TrimSpace(os.Getenv("ONCLAW_TURN_LOG_PATH"))
	date := when.UTC().Format("2006-01-02")
	y, m := date[0:4], date[5:7]
	file := fmt.Sprintf("%s.jsonl", date)
	sharded := func(root string) string {
		return filepath.Join(root, "logs", y, m, file)
	}
	if p == "" {
		return sharded(filepath.Join(layout.CWD, DotDir, "traces"))
	}
	if h, err := os.UserHomeDir(); err == nil {
		p = expandTilde(h, p)
	}
	p = filepath.Clean(p)
	if !filepath.IsAbs(p) {
		p = filepath.Join(layout.CWD, p)
	}
	if strings.HasSuffix(strings.ToLower(p), ".jsonl") {
		return p
	}
	return sharded(p)
}

func turnLogFileDisabled() bool {
	v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_TURN_LOG"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

var turnLogMu sync.Mutex

func appendTurnLogLine(layout Layout, when time.Time, payload any) {
	if turnLogFileDisabled() {
		return
	}
	path := TurnLogPathForDate(layout, when)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		slog.Warn("memory.turn_log.mkdir", "path", path, "err", err)
		return
	}
	line, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("memory.turn_log.marshal", "err", err)
		return
	}
	line = append(line, '\n')

	turnLogMu.Lock()
	defer turnLogMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("memory.turn_log.open", "path", path, "err", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		slog.Warn("memory.turn_log.write", "path", path, "err", err)
	}
}

func turnLogAssistantPreview(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) {
		return ""
	}
	var n int
	for i := range s {
		if n >= assistantTurnLogMaxRunes {
			return s[:i] + "…"
		}
		n++
	}
	return s
}

// AppendTurnToolLogJSONL appends one JSON line when a tool finishes (kind=tool).
func AppendTurnToolLogJSONL(layout Layout, sessionID, correlationID, userPreview string, entry loop.ToolTraceEntry) {
	now := time.Now()
	payload := struct {
		TS            string `json:"ts"`
		Kind          string `json:"kind"`
		SessionID     string `json:"session_id,omitempty"`
		CorrelationID string `json:"correlation_id,omitempty"`
		UserPreview   string `json:"user_preview,omitempty"`
		loop.ToolTraceEntry
	}{
		TS:             now.UTC().Format(time.RFC3339Nano),
		Kind:           "tool",
		SessionID:      sessionID,
		CorrelationID:  correlationID,
		UserPreview:    userPreview,
		ToolTraceEntry: entry,
	}
	appendTurnLogLine(layout, now, payload)
}

// AppendTurnAssistantFinalJSONL appends one JSON line for the user-visible final reply (kind=assistant_final).
func AppendTurnAssistantFinalJSONL(layout Layout, sessionID, correlationID, userPreview, assistantVisible string) {
	now := time.Now()
	payload := struct {
		TS               string `json:"ts"`
		Kind             string `json:"kind"`
		SessionID        string `json:"session_id,omitempty"`
		CorrelationID    string `json:"correlation_id,omitempty"`
		UserPreview      string `json:"user_preview,omitempty"`
		AssistantVisible string `json:"assistant_visible,omitempty"`
	}{
		TS:               now.UTC().Format(time.RFC3339Nano),
		Kind:             "assistant_final",
		SessionID:        sessionID,
		CorrelationID:    correlationID,
		UserPreview:      userPreview,
		AssistantVisible: turnLogAssistantPreview(assistantVisible),
	}
	appendTurnLogLine(layout, now, payload)
}
