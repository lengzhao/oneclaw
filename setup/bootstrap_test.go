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
	sk := filepath.Join(root, "skills", "skill-creator", "SKILL.md")
	if _, err := os.Stat(sk); err != nil {
		t.Fatalf("expected builtin skill-creator template: %v", err)
	}
	def := filepath.Join(root, "agents", "default.md")
	b, err := os.ReadFile(def)
	if err != nil {
		t.Fatalf("default.md: %v", err)
	}
	if !strings.Contains(string(b), root) {
		t.Fatalf("default.md should contain rendered UserDataRoot")
	}
}
