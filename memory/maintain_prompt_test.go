package memory

import (
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/prompts"
)

func TestMaintenanceSystemPromptData(t *testing.T) {
	d := MaintainPromptData{
		CWD:        "/proj/repo",
		Today:      "2026-04-05",
		MemoryPath: "/proj/repo/.oneclaw/memory/MEMORY.md",
		RunTS:      "2026-04-05T12:00:00Z",
	}
	got, err := prompts.Render(prompts.NameMaintenanceSystem, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{
		"silent memory indexer",
		"/proj/repo",
		"2026-04-05",
		"MEMORY.md",
		"2026-04-05T12:00:00Z",
		"header + bullets",
	} {
		if !strings.Contains(got, sub) {
			t.Fatalf("maintenance_system missing %q in:\n%s", sub, got)
		}
	}
}
