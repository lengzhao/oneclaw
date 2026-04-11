package prompts

import (
	"strings"
	"testing"
)

// maintenancePromptData mirrors memory.MaintainPromptData (avoid import cycle in this package test).
type maintenancePromptData struct {
	CWD, Today, MemoryPath, RulesMemoryPath, RunTS string
	DialogHistoryPath, WorkingTranscriptPath, TranscriptPath string
}

func TestRenderCompactEnvelope(t *testing.T) {
	const kind = "compact_boundary"
	const ts = "2006-01-02T15:04:05Z"
	summary := "line"
	want := "[oneclaw:" + kind + " ts=" + ts + "]\n" +
		"Earlier conversation (omitted from context for byte budget). Heuristic recap — verify with tools if needed:\n\n" +
		summary + "\n\n[/oneclaw:" + kind + "]\n"
	got, err := Render(NameCompactEnvelope, map[string]string{
		"Kind": kind, "Timestamp": ts, "Summary": summary,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderNilDataUsesEmptyStruct(t *testing.T) {
	_, err := Render(NameMaintenanceSystemPostTurn, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderEmptyName(t *testing.T) {
	_, err := Render("", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderUnknownTemplate(t *testing.T) {
	_, err := Render("does_not_exist", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderMaintenanceData(t *testing.T) {
	d := maintenancePromptData{
		CWD: "/p", Today: "2026-01-01",
		MemoryPath: "/p/.oneclaw/memory/2026-01-01.md", RulesMemoryPath: "/p/.oneclaw/memory/MEMORY.md",
		RunTS:                 "2026-01-01T00:00:00Z",
		DialogHistoryPath:     "/p/.oneclaw/memory/2026-01-01/dialog_history.json",
		WorkingTranscriptPath: "/p/.oneclaw/working_transcript.json",
		TranscriptPath:        "/p/.oneclaw/transcript.json",
	}
	got, err := Render(NameMaintenanceSystemPostTurn, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "/p") || !strings.Contains(got, "silent memory") || !strings.Contains(got, "post-turn") {
		t.Fatalf("got %q", got)
	}
	got2, err := Render(NameMaintenanceSystemScheduled, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got2, "scheduled") || !strings.Contains(got2, "far-field") {
		t.Fatalf("scheduled template missing scope: %q", got2)
	}
	if !strings.Contains(got2, "dialog_history.json") {
		t.Fatalf("scheduled template missing session paths: %q", got2)
	}
	if !strings.Contains(got2, "SKILL.md") || !strings.Contains(got2, "write_behavior_policy") {
		t.Fatalf("scheduled template missing skills guidance: %q", got2)
	}
}
