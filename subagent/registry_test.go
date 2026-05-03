package subagent

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
)

func TestBuildExecRegistry_emptyAllow_noRunAgent(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "w")
	deps := &RunAgentDeps{
		Turn:            TurnBinding{SessionSegment: "test-session"},
		Catalog:         &catalog.Catalog{},
		Cfg:             &config.File{},
		InstructionRoot: filepath.Join(tmp, "instructions"),
	}
	reg, err := BuildExecRegistry(ws, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	names := reg.Names()
	if slices.Contains(names, "run_agent") {
		t.Fatalf("run_agent should be opt-in: %v", names)
	}
	if !slices.Contains(names, "cron") {
		t.Fatalf("want cron in default registry: %v", names)
	}
	if !slices.Contains(names, "todo") {
		t.Fatalf("want todo in default registry: %v", names)
	}
	if !slices.Contains(names, "read_memory_month") {
		t.Fatalf("want read_memory_month in default registry: %v", names)
	}
}

func TestBuildExecRegistry_explicitRunAgent(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "w")
	deps := &RunAgentDeps{
		Turn:            TurnBinding{SessionSegment: "test-session"},
		Catalog:         &catalog.Catalog{},
		Cfg:             &config.File{},
		InstructionRoot: filepath.Join(tmp, "instructions"),
	}
	reg, err := BuildExecRegistry(ws, []string{"read_file", "run_agent"}, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(reg.Names(), "run_agent") {
		t.Fatalf("want run_agent in %v", reg.Names())
	}
}
