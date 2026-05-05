package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResetConversation removes only user-visible transcript files (*_transcript.jsonl and legacy transcript.jsonl) under sessionRoot.
// It does not delete runs/ (execution journal), subs/ (delegated runs), MEMORY.md, or workspace files —
// those stay as factual / audit state. The main ChatModelAgent replays transcript turns from default_transcript.jsonl (bounded);
// clearing it removes chat history from model context while keeping MEMORY.md / memory/ recall in the system prompt.
func ResetConversation(sessionRoot string) error {
	root := strings.TrimSpace(sessionRoot)
	if root == "" {
		return fmt.Errorf("session: empty session root")
	}
	legacy := filepath.Join(root, legacyTranscriptFile)
	if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("session: remove transcript: %w", err)
	}
	matches, err := filepath.Glob(filepath.Join(root, "*_transcript.jsonl"))
	if err != nil {
		return fmt.Errorf("session: glob transcript files: %w", err)
	}
	for _, p := range matches {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("session: remove transcript: %w", err)
		}
	}
	return nil
}
