package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/rtopts"
)

func TestTurnLogPathForDateDefault(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.TurnLogPath = ""
	rtopts.Set(&s)
	lay := Layout{CWD: "/proj/repo"}
	when := time.Date(2024, 3, 5, 15, 0, 0, 0, time.UTC)
	got := TurnLogPathForDate(lay, when)
	want := filepath.Join("/proj/repo", DotDir, "traces", "logs", "2024", "03", "2024-03-05.jsonl")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTurnLogPathForDateEnvSingleFile(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.TurnLogPath = "custom/trace.jsonl"
	rtopts.Set(&s)
	lay := Layout{CWD: "/proj/repo"}
	when := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	got := TurnLogPathForDate(lay, when)
	want := filepath.Join("/proj/repo", "custom", "trace.jsonl")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTurnLogPathForDateEnvAbsSingleFile(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.TurnLogPath = "/tmp/abs.jsonl"
	rtopts.Set(&s)
	lay := Layout{CWD: "/proj/repo"}
	got := TurnLogPathForDate(lay, time.Now())
	if got != "/tmp/abs.jsonl" {
		t.Fatalf("got %q", got)
	}
}

func TestTurnLogPathForDateEnvDirShard(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.TurnLogPath = "/var/log/oneclaw-traces"
	rtopts.Set(&s)
	lay := Layout{CWD: "/proj/repo"}
	when := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	got := TurnLogPathForDate(lay, when)
	want := filepath.Join("/var/log/oneclaw-traces", "logs", "2025", "12", "2025-12-01.jsonl")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAppendTurnToolLogJSONLLine(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.DisableTurnLog = false
	s.TurnLogPath = ""
	rtopts.Set(&s)
	cwd := t.TempDir()
	lay := Layout{CWD: cwd}
	AppendTurnToolLogJSONL(lay, "sess_x", "c1", "hi", loop.ToolTraceEntry{Step: 0, Name: "read_file", OK: true})
	path := TurnLogPathForDate(lay, time.Now())
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(raw))
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("%s: %v", line, err)
	}
	if rec["kind"] != "tool" || rec["session_id"] != "sess_x" || rec["name"] != "read_file" {
		t.Fatalf("%v", rec)
	}
}

func TestAppendTurnAssistantFinalJSONLLine(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.DisableTurnLog = false
	s.TurnLogPath = ""
	rtopts.Set(&s)
	cwd := t.TempDir()
	lay := Layout{CWD: cwd}
	AppendTurnAssistantFinalJSONL(lay, "sess_a", "c2", "user q", "final reply text")
	path := TurnLogPathForDate(lay, time.Now())
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(raw))
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("%s: %v", line, err)
	}
	if rec["kind"] != "assistant_final" || rec["assistant_visible"] != "final reply text" {
		t.Fatalf("%v", rec)
	}
}
