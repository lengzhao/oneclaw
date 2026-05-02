package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/tools"
)

func TestEngineSaveTranscriptTo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := tools.NewRegistry()
	e := NewEngine(dir, reg)
	p := filepath.Join(dir, "sub", "t.json")
	if err := e.SaveTranscriptTo(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected file: %v", err)
	}
}

func TestEngineSaveTranscriptTo_emptyPathNoOp(t *testing.T) {
	t.Parallel()
	e := NewEngine(t.TempDir(), tools.NewRegistry())
	if err := e.SaveTranscriptTo(""); err != nil {
		t.Fatal(err)
	}
}

func TestEngineSaveWorkingTranscriptTo_tailCapDefault30(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	e := NewEngine(dir, tools.NewRegistry())
	for i := 0; i < 40; i++ {
		e.Messages = append(e.Messages, schema.UserMessage("x"))
	}
	p := filepath.Join(dir, "w.json")
	if err := e.SaveWorkingTranscriptTo(p); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var wrap struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		t.Fatal(err)
	}
	if len(wrap.Messages) != 30 {
		t.Fatalf("tail cap: got %d messages, want 30", len(wrap.Messages))
	}
	if len(e.Messages) != 40 {
		t.Fatalf("in-memory messages should not be truncated, got %d", len(e.Messages))
	}
}

func TestEngineSaveWorkingTranscriptTo_unlimitedWhenNegative(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	e := NewEngine(dir, tools.NewRegistry())
	e.WorkingTranscriptMaxMessages = -1
	for i := 0; i < 35; i++ {
		e.Messages = append(e.Messages, schema.AssistantMessage("a", nil))
	}
	p := filepath.Join(dir, "w.json")
	if err := e.SaveWorkingTranscriptTo(p); err != nil {
		t.Fatal(err)
	}
	msgs, err := loop.UnmarshalMessages(mustRead(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 35 {
		t.Fatalf("unlimited: got %d messages, want 35", len(msgs))
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
