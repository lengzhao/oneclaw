package memory

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PostTurn runs optional memory maintenance after a successful turn (simplified extract / dream hook).
func PostTurn(layout Layout, lastUserText, lastAssistantVisible string) {
	if AutoMemoryDisabled() {
		return
	}
	if memoryExtractDisabled() {
		return
	}
	line := buildDailyLogLine(lastUserText, lastAssistantVisible)
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
	}
}

func buildDailyLogLine(user, assistant string) string {
	ts := time.Now().UTC().Format(time.RFC3339)
	u := oneLine(user, 200)
	a := oneLine(assistant, 200)
	return "- " + ts + " | user: " + u + " | assistant: " + a + "\n"
}

func oneLine(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// memoryExtractDisabled: daily log append is on by default; set ONCLAW_DISABLE_MEMORY_EXTRACT=1 to turn off.
func memoryExtractDisabled() bool {
	v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_MEMORY_EXTRACT"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}
