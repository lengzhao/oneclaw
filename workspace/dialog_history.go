package workspace

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/oneclaw/loop"
)

// maxDialogHistoryMessages caps stored messages (user+assistant pairs count as two). Oldest entries drop first.
var maxDialogHistoryMessages = 400

func trimDialogHistoryFront(msgs []json.RawMessage, max int) []json.RawMessage {
	if max <= 0 || len(msgs) <= max {
		return msgs
	}
	out := msgs
	for len(out) > max {
		if len(out) >= 2 {
			out = out[2:]
			continue
		}
		out = out[1:]
	}
	return out
}

// AppendDialogHistoryPair appends one slim user message and one assistant message to the day's dialog_history.json.
// sessionID selects a per-session file under the date directory when non-empty; otherwise the legacy single file path is used.
func AppendDialogHistoryPair(layout Layout, date, sessionID string, user, assistant *schema.Message) error {
	if layout.CWD == "" {
		return fmt.Errorf("workspace: empty layout cwd")
	}
	var path string
	if strings.TrimSpace(sessionID) != "" {
		path = layout.DialogHistoryPathForSession(date, sessionID)
	} else {
		path = layout.DialogHistoryPath(date)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir dialog history: %w", err)
	}

	var wrap loop.TranscriptJSON
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &wrap); err != nil {
			return fmt.Errorf("parse dialog history: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read dialog history: %w", err)
	}

	ub, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("marshal user message: %w", err)
	}
	ab, err := json.Marshal(assistant)
	if err != nil {
		return fmt.Errorf("marshal assistant message: %w", err)
	}
	wrap.Messages = append(wrap.Messages, ub, ab)
	before := len(wrap.Messages)
	wrap.Messages = trimDialogHistoryFront(wrap.Messages, maxDialogHistoryMessages)
	if len(wrap.Messages) < before {
		slog.Info("workspace.dialog_history.trimmed", "path", path, "dropped", before-len(wrap.Messages), "kept", len(wrap.Messages), "cap", maxDialogHistoryMessages)
	}

	out, err := json.MarshalIndent(wrap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dialog history: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return fmt.Errorf("write dialog history temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename dialog history: %w", err)
	}
	slog.Debug("workspace.dialog_history.appended", "path", path, "messages", len(wrap.Messages))
	return nil
}
