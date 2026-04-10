package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportSessionSnapshot(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	dot := filepath.Join(cwd, DotDir)
	if err := os.MkdirAll(filepath.Join(dot, "memory", "2026-04-10"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dot, "transcript.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dot, "memory", "2026-04-10", "x.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	artDir := filepath.Join(dot, "memory", "2026-04-10", "artifacts", "mcp")
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artDir, "big.txt"), []byte("skip-me"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	if err := ExportSessionSnapshot(cwd, out); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(out, DotDir, "transcript.json")
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("missing exported transcript: %v", err)
	}
	skip := filepath.Join(out, DotDir, "memory", "2026-04-10", "artifacts", "mcp", "big.txt")
	if _, err := os.Stat(skip); err == nil {
		t.Fatal("expected artifacts subtree to be skipped")
	}
}
