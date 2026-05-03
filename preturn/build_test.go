package preturn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/catalog"
)

func TestBuild_omitMemory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("SECRET_MEMORY"), 0o644); err != nil {
		t.Fatal(err)
	}
	ag := &catalog.Agent{AgentType: "x"}
	bundle, err := Build(dir, dir, ag, DefaultBudget(), &BuildOpts{OmitMemory: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(bundle.Instruction, "SECRET_MEMORY") {
		t.Fatalf("memory should be omitted: %q", bundle.Instruction)
	}
	bundle2, err := Build(dir, dir, ag, DefaultBudget(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bundle2.Instruction, "SECRET_MEMORY") {
		t.Fatalf("expected MEMORY in instruction: %q", bundle2.Instruction)
	}
}
