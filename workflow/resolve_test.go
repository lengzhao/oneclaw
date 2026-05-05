package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/config"
)

func TestResolveWorkflowPath_agentFilePreferred(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "custom.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "default.turn.yaml"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.File{}
	p, err := ResolveWorkflowPath(root, "custom", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "custom.yaml" {
		t.Fatal(p)
	}
}

func TestResolveWorkflowPath_fallbackDefaultTurn(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "default.turn.yaml"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.File{}
	p, err := ResolveWorkflowPath(root, "missing-agent", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "default.turn.yaml" {
		t.Fatal(p)
	}
}
