package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadTranscriptTurns_missingFile(t *testing.T) {
	got, err := LoadTranscriptTurns(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v len=%d", err, len(got))
	}
}

func TestLoadTranscriptTurns_trim(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	if err := AppendTranscriptTurn(dir, TranscriptTurn{Ts: time.Now(), Role: "user", Content: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := AppendTranscriptTurn(dir, TranscriptTurn{Ts: time.Now(), Role: "assistant", Content: "b"}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadTranscriptTurns(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Content != "a" || got[1].Content != "b" {
		t.Fatalf("got %+v", got)
	}
	large := make([]TranscriptTurn, 10)
	for i := range large {
		large[i] = TranscriptTurn{Role: "user", Content: string(rune('0' + i))}
	}
	trim := TrimTranscriptTail(large, 3)
	if len(trim) != 3 || trim[0].Content != "7" {
		t.Fatalf("got %+v", trim)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
