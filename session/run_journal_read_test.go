package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilterRunJournalByCorrelation(t *testing.T) {
	corr := "abc-123"
	raw := `{"ts":"2026-05-04T00:00:00Z","agent_type":"default","phase":"run_start","detail":{"correlation_id":"abc-123"}}
{"ts":"2026-05-04T00:00:01Z","agent_type":"default","phase":"run_complete","detail":{"correlation_id":"abc-123"}}
{"ts":"2026-05-04T00:00:02Z","agent_type":"default","phase":"run_start","detail":{"correlation_id":"other"}}
`
	out := FilterRunJournalByCorrelation([]byte(raw), corr)
	if !journalSnippetHasPhaseComplete(out) {
		t.Fatalf("want run_complete in filtered snippet:\n%s", out)
	}
	if !strings.Contains(out, "run_start") {
		t.Fatal("want run_start line preserved")
	}
}

func TestJournalLineMatchesCorrelation_falseOnMismatch(t *testing.T) {
	line := []byte(`{"detail":{"correlation_id":"x"}}`)
	if JournalLineMatchesCorrelation(line, "y") {
		t.Fatal("expected no match")
	}
}

func TestReadRunJournalText_prefersTurnFile(t *testing.T) {
	dir := t.TempDir()
	corr := "c1deadbeef"
	agent := "default"
	if err := os.MkdirAll(filepath.Join(dir, "runs", agent), 0o755); err != nil {
		t.Fatal(err)
	}
	turnPath := TurnRunJournalPath(dir, agent, corr)
	want := "{\"phase\":\"tool_call\",\"detail\":{\"correlation_id\":\"c1deadbeef\",\"tool_name\":\"x\"}}\n"
	if err := os.WriteFile(turnPath, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRunJournalText(dir, agent, corr, "current_turn")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("want turn file content, got %q", got)
	}
}
