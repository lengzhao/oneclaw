package observe

import (
	"os"
	"testing"
	"time"
)

func TestAppendExecJournal(t *testing.T) {
	path := t.TempDir() + "/e/exec.jsonl"
	rec := ExecRecord{Time: time.Unix(1, 0).UTC(), SessionID: "s", Phase: "p"}
	if err := AppendExecJournal(path, rec); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 10 {
		t.Fatalf("short file: %q", b)
	}
}
