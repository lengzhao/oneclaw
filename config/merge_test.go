package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMerged_layering(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p1 := filepath.Join(dir, "base.yaml")
	p2 := filepath.Join(dir, "overlay.yaml")
	if err := os.WriteFile(p1, []byte(`
sessions:
  isolate_instruction_root: false
default_model: openai_compatible/gpt-4o
models:
  - id: default
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte(`
models:
  - id: default
    base_url: https://example.com/v1
`), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := LoadMerged([]string{p1, p2})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Models) != 1 {
		t.Fatalf("models len %d", len(f.Models))
	}
	m0 := f.Models[0]
	if m0.BaseURL != "https://example.com/v1" {
		t.Fatalf("base_url=%q", m0.BaseURL)
	}
	if strings.TrimSpace(m0.DefaultModel) != "" {
		t.Fatalf("profile default_model should be unset: %q", m0.DefaultModel)
	}
	if strings.TrimSpace(f.DefaultModel) != "openai_compatible/gpt-4o" {
		t.Fatalf("root default_model=%q", f.DefaultModel)
	}
	if f.IsolateInstructionOrDefault() {
		t.Fatal("expected isolate false from base")
	}
}

func TestLoadMerged_emptyIsDefaults(t *testing.T) {
	t.Parallel()
	f, err := LoadMerged(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Models) != 1 {
		t.Fatalf("want 1 implicit profile, got %d", len(f.Models))
	}
	if !f.IsolateInstructionOrDefault() {
		t.Fatal("default isolate should be true")
	}
	if strings.TrimSpace(f.Models[0].DefaultModel) != "" {
		t.Fatalf("implicit profile must not set default_model: %q", f.Models[0].DefaultModel)
	}
	if strings.TrimSpace(f.DefaultModel) != "openai_compatible/gpt-5.4-nano" {
		t.Fatalf("root default_model %q", f.DefaultModel)
	}
}

func TestLoadMerged_modelsList(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "m.yaml")
	yaml := `
default_model: primary/gpt-4o-mini
models:
  - id: primary
    priority: 0
    api_key_env: OPENAI_API_KEY
  - id: backup
    priority: 10
    provider: mock
`
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := LoadMerged([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	def, err := ResolveModelProfile(f, "")
	if err != nil || def.ID != "primary" {
		t.Fatalf("default profile %+v %v", def, err)
	}
	pprof, err := ResolveModelProfile(f, "primary")
	if err != nil || pprof.ID != "primary" {
		t.Fatalf("primary %+v %v", pprof, err)
	}
	order := OrderedModelProfiles(f)
	if len(order) != 2 || order[0].ID != "primary" || order[1].ID != "backup" {
		t.Fatalf("order %v", order)
	}
}

func TestValidate_duplicateID(t *testing.T) {
	f := &File{
		Models: []ModelProfile{
			{ID: "x"},
			{ID: "x"},
		},
	}
	ApplyDefaults(f)
	if err := Validate(f); err == nil {
		t.Fatal("want duplicate error")
	}
}

func TestLoadMerged_clawbridgeSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(p, []byte(`
models:
  - id: default
clawbridge:
  clients:
    - id: w
      driver: webchat
      enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := LoadMerged([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Clawbridge.Clients) != 1 || f.Clawbridge.Clients[0].ID != "w" || !f.Clawbridge.Clients[0].Enabled {
		t.Fatalf("clawbridge clients: %+v", f.Clawbridge.Clients)
	}
}
