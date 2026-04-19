package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverProjectInstructions_UsesProjectRootWithoutDotDir(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(filepath.Join(root, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, AgentInstructionsFile), []byte("project agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "rules", "style.md"), []byte("project rule"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := DiscoverProjectInstructions(nested)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(got), got)
	}
	if got[0].Path != filepath.Join(root, AgentInstructionsFile) {
		t.Fatalf("first path = %q", got[0].Path)
	}
	if got[1].Path != filepath.Join(root, "rules", "style.md") {
		t.Fatalf("second path = %q", got[1].Path)
	}
}
