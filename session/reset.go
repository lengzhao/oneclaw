package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResetConversation removes only the user-visible chat transcript (transcript.jsonl) under sessionRoot.
// It does not delete runs/ (execution journal), subs/ (delegated runs), MEMORY.md, or workspace files —
// those stay as factual / audit state. The main ChatModelAgent replays transcript turns from this file (bounded);
// clearing it removes chat history from model context while keeping MEMORY.md / memory/ recall in the system prompt.
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
