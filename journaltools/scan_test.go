package journaltools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanJournalFile_countsDistinctToolCallsAndSkills(t *testing.T) {
	dir := t.TempDir()
	ud := filepath.Join(dir, "ud")
	path := filepath.Join(dir, "j.jsonl")
	content := `{"phase":"tool_call","detail":{"tool_call_id":"a","tool_name":"read_file","arguments":"{\"path\":\"skills/x/SKILL.md\"}"}}
{"phase":"tool_call","detail":{"tool_call_id":"a","tool_name":"read_file","arguments":"{\"path\":\"skills/x/SKILL.md\"}"}}
{"phase":"tool_call","detail":{"tool_call_id":"b","tool_name":"echo","arguments":"{}"}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ScanJournalFile(path, ud, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.DistinctToolCalls != 2 {
		t.Fatalf("distinct=%d want 2", m.DistinctToolCalls)
	}
	if m.ToolCallEvents != 3 {
		t.Fatalf("tool_call_events=%d want 3", m.ToolCallEvents)
	}
	if !m.SkillTreeToolUsed {
		t.Fatal("expected skill_tree_tool_used")
	}
	if m.ToolNameCounts["read_file"] != 1 || m.ToolNameCounts["echo"] != 1 {
		t.Fatalf("tool_name_counts=%v", m.ToolNameCounts)
	}
}

func TestScanJournalFile_durationAndPhases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "j.jsonl")
	content := `{"ts":"2026-05-04T00:00:00Z","phase":"run_start","detail":{}}
{"ts":"2026-05-04T00:00:10Z","phase":"tool_call","detail":{"tool_call_id":"a","tool_name":"read_file","arguments":"{}"}}
{"ts":"2026-05-04T00:00:20Z","phase":"run_complete","detail":{}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ScanJournalFile(path, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !m.HasRunStart || !m.HasRunComplete {
		t.Fatalf("phases start=%v complete=%v", m.HasRunStart, m.HasRunComplete)
	}
	if m.LineCount != 3 {
		t.Fatalf("line_count=%d want 3", m.LineCount)
	}
	if m.DurationSeconds < 19.9 || m.DurationSeconds > 20.1 {
		t.Fatalf("duration_seconds=%v want ~20", m.DurationSeconds)
	}
}

func TestScanJournalFile_matchTools(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "j.jsonl")
	content := `{"phase":"tool_call","detail":{"tool_call_id":"a","tool_name":"read_file","arguments":"{}"}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ScanJournalFile(path, "", &ScanOpts{MatchTools: []string{"read_file", "write_file"}})
	if err != nil {
		t.Fatal(err)
	}
	if !m.NamedToolsPresent["read_file"] || m.NamedToolsPresent["write_file"] {
		t.Fatalf("named present=%v", m.NamedToolsPresent)
	}
	if m.NamedToolsCounts["read_file"] != 1 || m.NamedToolsCounts["write_file"] != 0 {
		t.Fatalf("named counts=%v", m.NamedToolsCounts)
	}
}

func TestScanJournalFile_readSkillMarksSkillTreeUsed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "j.jsonl")
	content := `{"phase":"tool_call","detail":{"tool_call_id":"a","tool_name":"read_skill","arguments":"{\"skill_id\":\"demo\"}"}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ScanJournalFile(path, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !m.SkillTreeToolUsed {
		t.Fatal("expected skill_tree_tool_used for read_skill")
	}
}
