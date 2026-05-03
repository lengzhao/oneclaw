package subagent

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
)

func TestBuildExecRegistry_emptyAllow_noRunAgent(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "w")
	deps := &RunAgentDeps{
		Catalog: &catalog.Catalog{},
		Cfg:     &config.File{},
	}
	reg, err := BuildExecRegistry(ws, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	names := reg.Names()
	if slices.Contains(names, "run_agent") {
		t.Fatalf("run_agent should be opt-in: %v", names)
	}
}

func TestBuildExecRegistry_explicitRunAgent(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "w")
	deps := &RunAgentDeps{
		Catalog: &catalog.Catalog{},
		Cfg:     &config.File{},
	}
	reg, err := BuildExecRegistry(ws, []string{"echo", "run_agent"}, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(reg.Names(), "run_agent") {
		t.Fatalf("want run_agent in %v", reg.Names())
	}
}
