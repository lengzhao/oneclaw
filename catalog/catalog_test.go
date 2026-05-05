package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_builtinAgentsEmbedded(t *testing.T) {
	cat, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	for _, stem := range []string{"default", "memory_extractor", "skill_generator"} {
		if cat.Get(stem) == nil {
			t.Fatalf("missing builtin %q", stem)
		}
	}
	if !cat.Get("memory_extractor").ContextProfile.Disabled("memory_recall") {
		t.Fatalf("memory_extractor should disable memory_recall by default")
	}
	if !cat.Get("skill_generator").ContextProfile.Disabled("transcript") {
		t.Fatalf("skill_generator should disable transcript by default")
	}
}

func TestLoad_skipsReadmeMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# doc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foo.readme.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "worker.md"), []byte("plain body"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Get("README") != nil || cat.Get("foo") != nil {
		t.Fatalf("readme entries loaded: %+v", cat.Agents)
	}
	if cat.Get("worker") == nil {
		t.Fatal("expected worker from worker.md")
	}
}

func TestLoad_userDirWithoutEvolutionAgentsFallsBackToEmbeddedBuiltins(t *testing.T) {
	dir := t.TempDir()
	// Simulate user directory where only default agent is present; evolution agents were deleted.
	if err := os.WriteFile(filepath.Join(dir, "default.md"), []byte("user default body"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	mem := cat.Get("memory_extractor")
	if mem == nil {
		t.Fatal("missing embedded fallback memory_extractor")
	}
	skill := cat.Get("skill_generator")
	if skill == nil {
		t.Fatal("missing embedded fallback skill_generator")
	}
	if !mem.ContextProfile.Disabled("memory_recall") {
		t.Fatal("embedded fallback memory_extractor should disable memory_recall")
	}
	if !skill.ContextProfile.Disabled("transcript") {
		t.Fatal("embedded fallback skill_generator should disable transcript")
	}
}

func TestLoad_userDirWithoutDefaultFallsBackToEmbeddedDefault(t *testing.T) {
	dir := t.TempDir()
	// Simulate user directory where default.md was deleted.
	if err := os.WriteFile(filepath.Join(dir, "worker.md"), []byte("user worker body"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	def := cat.Get("default")
	if def == nil {
		t.Fatal("missing embedded fallback default")
	}
	if def.Name == "" {
		t.Fatal("embedded fallback default should be parsed")
	}
	if cat.Get("worker") == nil {
		t.Fatal("expected user worker to still be loaded")
	}
}
