package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendTranscriptTurn(t *testing.T) {
	dir := t.TempDir()
	if err := AppendTranscriptTurn(dir, TranscriptTurn{
		Ts: time.Unix(1, 0).UTC(), Role: "user", Content: "hi",
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "transcript.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 5 {
		t.Fatalf("%q", b)
	}
}
