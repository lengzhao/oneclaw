package memory

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/openai/openai-go"
)

// AppendDialogHistoryPair appends one slim user message and one assistant message to the day's dialog_history.json.
func AppendDialogHistoryPair(layout Layout, date string, user, assistant openai.ChatCompletionMessageParamUnion) error {
	if layout.CWD == "" {
		return fmt.Errorf("memory: empty layout cwd")
	}
	path := layout.DialogHistoryPath(date)
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
	slog.Debug("memory.dialog_history.appended", "path", path, "messages", len(wrap.Messages))
	return nil
}
