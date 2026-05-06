package workflow

import (
	"fmt"
	"strings"
)

// TranscriptSummaryMode reports whether *_transcript.jsonl should store short placeholder
// lines instead of full user/assistant text. Run Journal still records full content.
//
// Workflow document meta (optional):
//   - transcript_mode: summary — enable summary lines
//   - transcript_summary: true — alias for the same behavior
func TranscriptSummaryMode(meta map[string]any) bool {
	if len(meta) == 0 {
		return false
	}
	if v, ok := meta["transcript_mode"]; ok {
		s := strings.TrimSpace(strings.ToLower(fmt.Sprint(v)))
		return s == "summary"
	}
	if v, ok := meta["transcript_summary"]; ok {
		return metaTruthy(v)
	}
	return false
}

func metaTruthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		return s == "1" || s == "true" || s == "yes" || s == "on"
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	default:
		return false
	}
}
