package session

import (
	"os"
	"path/filepath"
	"testing"

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
