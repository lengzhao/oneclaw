package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMergeYAMLMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	existing := []byte(`sessions:
  isolate_instruction_root: false
`)
	if err := os.WriteFile(path, existing, 0o644); err != nil {
		t.Fatal(err)
	}
	defaults := []byte(`user_data_root: /tmp/x
models:
  - id: default
    priority: 0
`)
	if err := MergeYAMLMissingFile(path, defaults); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["user_data_root"]; !ok {
		t.Fatal("expected injected user_data_root")
	}
	sessions, _ := m["sessions"].(map[string]any)
	v, ok := sessions["isolate_instruction_root"].(bool)
	if !ok || v != false {
		t.Fatalf("should preserve isolate_instruction_root false: %+v", sessions)
	}
}

func TestMergeYAMLMissingFile_preservesScalarLineComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	existing := []byte("user_data_root: /mine # keep-comment\nsessions:\n  isolate_instruction_root: false\n")
	if err := os.WriteFile(path, existing, 0o644); err != nil {
		t.Fatal(err)
	}
	defaults := []byte(`models:
  - id: default
    priority: 0
`)
	if err := MergeYAMLMissingFile(path, defaults); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "keep-comment") {
		t.Fatalf("lost comment: %s", raw)
	}
}
