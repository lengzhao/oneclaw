package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMergeInitYAMLPreservesUserScalars(t *testing.T) {
	tmpl := []byte(`
model: gpt-4o
openai:
  api_key: ""
  base_url: ""
chat:
  transport: auto
`)
	exist := []byte(`
model: my-model
openai:
  api_key: "secret"
`)
	merged, changed, err := mergeInitYAML(tmpl, exist)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected merge to add missing keys")
	}
	var out map[string]any
	if err := yaml.Unmarshal(merged, &out); err != nil {
		t.Fatal(err)
	}
	if out["model"] != "my-model" {
		t.Fatalf("model: got %v", out["model"])
	}
	oa := out["openai"].(map[string]any)
	if oa["api_key"] != "secret" {
		t.Fatalf("api_key: got %v", oa["api_key"])
	}
	if oa["base_url"] != "" {
		t.Fatalf("base_url: want empty default from template, got %q", oa["base_url"])
	}
	if _, ok := out["chat"]; !ok {
		t.Fatal("expected chat from template")
	}
}

func TestMergeInitYAMLNoChangeWhenComplete(t *testing.T) {
	tmpl := []byte(`model: gpt-4o
chat:
  transport: auto
`)
	exist := []byte(`model: gpt-4o
chat:
  transport: auto
`)
	_, changed, err := mergeInitYAML(tmpl, exist)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected no change")
	}
}

func TestMergeInitYAMLKeepsUserSlice(t *testing.T) {
	tmpl := []byte(`
channels:
  - id: localweb
    type: statichttp
`)
	exist := []byte(`
channels:
  - id: only-cli
    type: cli
`)
	merged, changed, err := mergeInitYAML(tmpl, exist)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("user already had channels key; list must not be replaced")
	}
	if merged != nil {
		t.Fatal("expected nil merged bytes when unchanged")
	}
}

func TestMergeInitYAMLInvalidExisting(t *testing.T) {
	tmpl := []byte(`model: x`)
	exist := []byte(`openai: [`)
	_, _, err := mergeInitYAML(tmpl, exist)
	if err == nil {
		t.Fatal("expected error")
	}
}
