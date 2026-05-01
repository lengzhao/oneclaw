package session

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/workspace"
)

// bindExecutionLogShard pins one NDJSON file for this user turn:
//   <session-runtime>/execution/<agent_id>/<YYYY-MM-DD>/<turn_id>.jsonl
// (same anchor rules as tasks.json via JoinSessionWorkspaceWithInstruction).
func (e *Engine) bindExecutionLogShard(turnID string) {
	if e == nil {
		return
	}
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		e.executionLogTurnID = ""
		e.executionLogDay = ""
		e.executionLogRel = ""
		return
	}
	day := time.Now().Format("2006-01-02")
	agent := sanitizeExecutionPathSegment(e.EffectiveRootAgentID())
	file := sanitizeExecutionPathSegment(turnID) + ".jsonl"
	e.executionLogTurnID = turnID
	e.executionLogDay = day
	e.executionLogRel = ""
	if strings.TrimSpace(e.CWD) != "" {
		e.executionLogRel = filepath.Join("execution", agent, day, file)
		if ur := strings.TrimSpace(e.UserDataRoot); ur != "" {
			abs := e.executionLogPath()
			if abs != "" {
				if rel, err := filepath.Rel(filepath.Clean(ur), filepath.Clean(abs)); err == nil && rel != "" && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
					e.executionLogRel = rel
				}
			}
		}
	}
}

func sanitizeExecutionPathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || out == "." || out == ".." {
		return "unknown"
	}
	return out
}

// executionLogPath returns the append-only JSONL path for this bound turn (empty if not bound or no cwd).
func (e *Engine) executionLogPath() string {
	if e == nil || strings.TrimSpace(e.executionLogTurnID) == "" || strings.TrimSpace(e.CWD) == "" {
		return ""
	}
	agent := sanitizeExecutionPathSegment(e.EffectiveRootAgentID())
	day := strings.TrimSpace(e.executionLogDay)
	if day == "" {
		day = time.Now().Format("2006-01-02")
	}
	turnFile := sanitizeExecutionPathSegment(e.executionLogTurnID) + ".jsonl"
	return workspace.JoinSessionWorkspaceWithInstruction(e.CWD, e.InstructionRoot, e.WorkspaceFlat,
		"execution", agent, day, turnFile)
}

func (e *Engine) execJournalWanted() bool {
	return e != nil && e.executionLogPath() != ""
}

func (e *Engine) wantsLifecycle() bool {
	return e.hasNotify() || e.execJournalWanted()
}

// appendExecutionRecord appends one JSON object per line (best-effort; never panics).
func (e *Engine) appendExecutionRecord(ctx context.Context, rec map[string]any) {
	if e == nil || !e.execJournalWanted() {
		return
	}
	path := e.executionLogPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		slog.Warn("session.execution_log.mkdir", "path", path, "err", err)
		return
	}
	line := map[string]any{
		"ts":              time.Now().UnixMilli(),
		"schema_version":  1,
		"session_id":      e.SessionID,
		"interaction_day": strings.TrimSpace(e.executionLogDay),
	}
	if r := strings.TrimSpace(e.executionLogRel); r != "" {
		line["execution_log"] = r
	}
	for k, v := range rec {
		line[k] = v
	}
	b, err := json.Marshal(line)
	if err != nil {
		slog.Warn("session.execution_log.marshal", "err", err)
		return
	}
	e.execMu.Lock()
	defer e.execMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("session.execution_log.open", "path", path, "err", err)
		return
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		slog.Warn("session.execution_log.write", "path", path, "err", err)
	}
	if err := f.Close(); err != nil {
		slog.Warn("session.execution_log.close", "path", path, "err", err)
	}
	_ = ctx
}

func toolCallEndRecord(ent loop.ToolTraceEntry) map[string]any {
	m := map[string]any{
		"record":         "tool_call_end",
		"model_step":     ent.Step,
		"name":           ent.Name,
		"ok":             ent.OK,
		"duration_ms":    ent.DurationMs,
		"args_preview":   ent.ArgsPreview,
		"out_preview":    ent.OutPreview,
	}
	if ent.ToolUseID != "" {
		m["tool_use_id"] = ent.ToolUseID
	}
	if ent.Err != "" {
		m["err"] = ent.Err
	}
	return m
}
