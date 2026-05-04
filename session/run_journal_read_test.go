package session

import (
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
