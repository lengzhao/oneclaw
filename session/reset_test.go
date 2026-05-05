package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResetConversation(t *testing.T) {
	dir := t.TempDir()
	tr := filepath.Join(dir, "default_transcript.jsonl")
	if err := os.WriteFile(tr, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tr2 := filepath.Join(dir, "memory_extractor_transcript.jsonl")
	if err := os.WriteFile(tr2, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runs := filepath.Join(dir, "runs", "default")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runs, "runs.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	subs := filepath.Join(dir, "subs", "sub-1")
	if err := os.MkdirAll(subs, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ResetConversation(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tr); !os.IsNotExist(err) {
		t.Fatal("expected transcript removed")
	}
	if _, err := os.Stat(tr2); !os.IsNotExist(err) {
		t.Fatal("expected agent transcript removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "runs")); err != nil {
		t.Fatal("expected runs/ preserved", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "subs")); err != nil {
		t.Fatal("expected subs/ preserved", err)
	}
}
