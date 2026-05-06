package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBootstrap_idempotent(t *testing.T) {
	root := t.TempDir()
	if err := Bootstrap(root); err != nil {
		t.Fatal(err)
	}
	if err := Bootstrap(root); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["models"]; !ok {
		t.Fatal("missing models")
	}
	readme := filepath.Join(root, "skills", "README.md")
	if _, err := os.Stat(readme); err != nil {
		t.Fatalf("expected skills/README.md from bootstrap: %v", err)
	}
	sk := filepath.Join(root, "skills", "skill-creator", "SKILL.md")
	if _, err := os.Stat(sk); err != nil {
		t.Fatalf("expected bundled skill-creator (core capability loop): %v", err)
	}
	def := filepath.Join(root, "agents", "default.md")
	b, err := os.ReadFile(def)
	if err != nil {
		t.Fatalf("default.md: %v", err)
	}
	if !strings.Contains(string(b), root) {
		t.Fatalf("default.md should contain rendered UserDataRoot")
	}
	for _, name := range []string{"AGENT.md", "MEMORY.md", "SOUL.md", "USER.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("expected root instruction template %s copied from embedded templates/: %v", name, err)
		}
	}
	agReadme := filepath.Join(root, "agents", "README.md")
	if _, err := os.Stat(agReadme); err != nil {
		t.Fatalf("expected agents/README.md copied from embedded templates/: %v", err)
	}
	for _, p := range []string{
		filepath.Join(root, "agents", "memory_extractor.md"),
		filepath.Join(root, "agents", "skill_generator.md"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected agent template copied from embedded templates/: %s: %v", p, err)
		}
	}
	for _, dir := range []string{
		filepath.Join(root, "sessions"),
		filepath.Join(root, "prompts"),
		filepath.Join(root, "knowledge", "sources"),
		filepath.Join(root, "key_files"),
	} {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			t.Fatalf("expected init directory %s: %v", dir, err)
		}
	}
}

func TestSyncWorkflowTemplatesFromEmbed_overwritesExistingWorkflowYAML(t *testing.T) {
	root := t.TempDir()
	if err := Bootstrap(root); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(root, "workflows", "skill_generator.yaml")
	if err := os.WriteFile(skillPath, []byte("workflow_spec_version: 2\nid: stale.stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SyncWorkflowTemplatesFromEmbed(root); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if strings.Contains(body, "stale.stub") {
		t.Fatal("expected embedded skill_generator to overwrite stale file")
	}
	if !strings.Contains(body, "skill_generator.turn") {
		t.Fatal("expected embedded skill_generator.turn id in synced file")
	}
	if !strings.Contains(body, "require_truthy") {
		t.Fatal("expected synced skill_generator template to include require_truthy gate on llm")
	}
}
