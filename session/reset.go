package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResetConversation removes only the user-visible chat transcript (transcript.jsonl) under sessionRoot.
// It does not delete runs/ (execution journal), subs/ (delegated runs), MEMORY.md, or workspace files —
// those stay as factual / audit state; the main agent turn already uses a single-turn prompt (no transcript replay).
func ResetConversation(sessionRoot string) error {
	root := strings.TrimSpace(sessionRoot)
	if root == "" {
		return fmt.Errorf("session: empty session root")
	}
	transcript := filepath.Join(root, "transcript.jsonl")
	if err := os.Remove(transcript); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("session: remove transcript: %w", err)
	}
	return nil
}
