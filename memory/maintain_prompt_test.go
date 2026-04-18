package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/prompts"
)

func TestMaintenanceSystemPromptData(t *testing.T) {
	d := MaintainPromptData{
		CWD:             "/proj/repo",
		Today:           "2026-04-05",
		MemoryPath:      "/proj/repo/.oneclaw/memory/2026-04-05.md",
		RulesMemoryPath: "/proj/repo/.oneclaw/memory/MEMORY.md",
		RunTS:           "2026-04-05T12:00:00Z",
	}
	got, err := prompts.Render(prompts.NameMaintenanceSystemPostTurn, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{
		"silent memory indexer",
		"post-turn",
		"/proj/repo",
		"2026-04-05",
		"2026-04-05.md",
		"MEMORY.md",
		"2026-04-05T12:00:00Z",
		"header + bullets",
		"Tool trace is authoritative",
		"tools (this turn)",
		"Skills promotion",
		"write_behavior_policy",
	} {
		if !strings.Contains(got, sub) {
			t.Fatalf("maintenance_system missing %q in:\n%s", sub, got)
		}
	}
}

func TestUserMaintainSystemFileReplacesEmbedded(t *testing.T) {
	dir := t.TempDir()
	custom := "USER_CUSTOM {{.Today}} {{.MemoryPath}} end."
	if err := os.WriteFile(filepath.Join(dir, MaintainScheduledSystemFile), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	layout := Layout{
		InstructionRoot: dir,
		CWD:             filepath.Join(dir, "workspace"),
		Project:         filepath.Join(dir, "memory"),
	}
	got := maintenanceSystemPromptForPathway(pathwayScheduled, layout, "/x/2026-04-05.md", "/x/MEMORY.md", "2026-04-05", "2026-04-05T12:00:00Z")
	if !strings.Contains(got, "USER_CUSTOM") {
		t.Fatalf("want user file content, got:\n%s", got)
	}
	if strings.Contains(got, "silent memory indexer") {
		t.Fatalf("user file should replace embedded prompt, still got default phrase")
	}
}

func TestUserMaintainSystemFileInvalidFallsBackToEmbedded(t *testing.T) {
	dir := t.TempDir()
	// Broken template → parse/execute fails → fall back to embedded.
	if err := os.WriteFile(filepath.Join(dir, MaintainPostTurnSystemFile), []byte("broken {{"), 0o644); err != nil {
		t.Fatal(err)
	}
	layout := Layout{
		InstructionRoot: dir,
		CWD:             filepath.Join(dir, "workspace"),
		Project:         filepath.Join(dir, "memory"),
	}
	got := maintenanceSystemPromptForPathway(pathwayPostTurn, layout, "/x/2026-04-05.md", "/x/MEMORY.md", "2026-04-05", "2026-04-05T12:00:00Z")
	if !strings.Contains(got, "silent memory indexer") {
		t.Fatalf("invalid user template should fall back to embedded, got:\n%s", got)
	}
}
