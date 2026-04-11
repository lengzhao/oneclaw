package loop

import (
	"strings"
	"sync"
	"unicode/utf8"
)

// ToolTraceEntry is a slim record of one tool invocation in a turn.
// It is not injected into model context; callers use it for memory extract / offline logs.
type ToolTraceEntry struct {
	Step        int    `json:"step"`
	ToolUseID   string `json:"tool_use_id,omitempty"`
	Name        string `json:"name"`
	OK          bool   `json:"ok"`
	Err         string `json:"err,omitempty"`
	ArgsPreview string `json:"args_preview,omitempty"`
	OutPreview  string `json:"out_preview,omitempty"`
	DurationMs  int64  `json:"duration_ms,omitempty"`
}

const toolTraceArgsMaxRunes = 120
const toolTraceOutMaxRunes = 200
const toolTraceErrMaxRunes = 400

// ToolTraceSink collects ToolTraceEntry values from a single RunTurn (safe for concurrent tool batches).
type ToolTraceSink struct {
	mu sync.Mutex
	e  []ToolTraceEntry
}

// Add appends one entry (used from runOneTool; order follows completion order when tools run in parallel).
func (s *ToolTraceSink) Add(entry ToolTraceEntry) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.e = append(s.e, entry)
}

// Snapshot returns a copy of collected entries.
func (s *ToolTraceSink) Snapshot() []ToolTraceEntry {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ToolTraceEntry, len(s.e))
	copy(out, s.e)
	return out
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	s = trimSpaceOneLine(s)
	var n int
	for i := range s {
		if n == max {
			return s[:i] + "…"
		}
		n++
	}
	return s
}

func trimSpaceOneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}

func previewArgsJSON(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	s := string(raw)
	if !utf8.ValidString(s) {
		return ""
	}
	return truncateRunes(s, toolTraceArgsMaxRunes)
}

func previewToolOut(s string) string {
	if s == "" {
		return ""
	}
	if !utf8.ValidString(s) {
		return ""
	}
	one := trimSpaceOneLine(s)
	runes := []rune(one)
	if len(runes) <= toolTraceOutMaxRunes {
		return string(runes)
	}
	const sep = " … "
	sepR := []rune(sep)
	remain := toolTraceOutMaxRunes - len(sepR)
	if remain < 8 {
		return truncateRunes(one, toolTraceOutMaxRunes)
	}
	headLen := remain / 2
	tailLen := remain - headLen
	head := string(runes[:headLen])
	tail := string(runes[len(runes)-tailLen:])
	return head + sep + tail
}
