package memory

import (
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
