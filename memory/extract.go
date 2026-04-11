package memory

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/rtopts"
)

// PostTurnInput is one user turn's extract payload (tools are not added to chat history).
type PostTurnInput struct {
	SessionID        string
	CorrelationID    string
	UserText         string
	AssistantVisible string
	Tools            []loop.ToolTraceEntry
}

// MemoryExtractEnabled mirrors PostTurn gating: auto memory on and extract not disabled.
func MemoryExtractEnabled() bool {
	return !AutoMemoryDisabled() && !memoryExtractDisabled()
}

// PostTurn runs optional memory maintenance after a successful turn (simplified extract / dream hook).
func PostTurn(layout Layout, in PostTurnInput) {
	if AutoMemoryDisabled() {
		return
	}
	if memoryExtractDisabled() {
		return
	}
	line := buildDailyLogLine(in.UserText, in.AssistantVisible, in.Tools)
	path := DailyLogPath(layout.Auto, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		slog.Warn("memory.daily_log.mkdir", "path", path, "err", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("memory.daily_log.open", "path", path, "err", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		slog.Warn("memory.daily_log.write", "path", path, "err", err)
		return
	}
	AppendMemoryAudit(layout, path, "daily_log_line", []byte(line))
}

func buildDailyLogLine(user, assistant string, tools []loop.ToolTraceEntry) string {
	ts := time.Now().UTC().Format(time.RFC3339)
	u := oneLine(user, 200)
	a := oneLine(assistant, 200)
	s := "- " + ts + " | user: " + u + " | assistant: " + a
	if sum := formatToolSummary(tools); sum != "" {
		s += " | tools: " + sum
	}
	return s + "\n"
}

func formatToolSummary(entries []loop.ToolTraceEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(e.Name)
		if e.OK {
			b.WriteString(":ok")
		} else {
			b.WriteString(":err")
			if e.Err != "" {
				b.WriteString("(")
				b.WriteString(oneLine(e.Err, 48))
				b.WriteString(")")
			}
		}
	}
	return b.String()
}

func oneLine(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func memoryExtractDisabled() bool {
	return rtopts.Current().DisableMemoryExtract
}
