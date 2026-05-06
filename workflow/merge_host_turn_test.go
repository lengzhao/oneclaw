package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeHostTurnNodesFromSkillGenerator_hostTurnSteps(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillYAML := `workflow_spec_version: 2
id: skill_generator.turn
steps:
  - use: noop
  - id: skill_stats
    host_turn: true
    use: noop
  - id: skill_gen_if
    host_turn: true
    use: noop
    depends_on:
      - skill_stats
`
	if err := os.WriteFile(filepath.Join(wfDir, "skill_generator.yaml"), []byte(skillYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	host := &Workflow{
		SpecVersion: 2,
		ID:          "default.turn",
		Nodes: map[string]Node{
			"receive": {Use: "on_receive"},
		},
	}
	if err := MergeHostTurnNodesFromSkillGenerator(root, host); err != nil {
		t.Fatal(err)
	}
	if _, ok := host.Nodes["skill_stats"]; !ok {
		t.Fatal("expected merged skill_stats")
	}
	if _, ok := host.Nodes["skill_gen_if"]; !ok {
		t.Fatal("expected merged skill_gen_if")
	}
}

func TestMergeHostTurnNodesFromSkillGenerator_hostTurnNodesMap(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillYAML := `workflow_spec_version: 2
id: skill_generator.turn
host_turn_nodes:
  skill_stats:
    use: noop
steps:
  - use: noop
`
	if err := os.WriteFile(filepath.Join(wfDir, "skill_generator.yaml"), []byte(skillYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	host := &Workflow{SpecVersion: 2, ID: "default.turn", Nodes: map[string]Node{}}
	if err := MergeHostTurnNodesFromSkillGenerator(root, host); err != nil {
		t.Fatal(err)
	}
	if _, ok := host.Nodes["skill_stats"]; !ok {
		t.Fatal("expected host_turn_nodes fallback merge")
	}
}

func TestMergeHostTurnNodesFromSkillGenerator_legacyMetaString(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillYAML := `workflow_spec_version: 2
id: skill_generator.turn
meta:
  host_turn_nodes_yaml: |
    skill_stats:
      use: noop
steps:
  - use: noop
`
	if err := os.WriteFile(filepath.Join(wfDir, "skill_generator.yaml"), []byte(skillYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	host := &Workflow{SpecVersion: 2, ID: "default.turn", Nodes: map[string]Node{}}
	if err := MergeHostTurnNodesFromSkillGenerator(root, host); err != nil {
		t.Fatal(err)
	}
	if _, ok := host.Nodes["skill_stats"]; !ok {
		t.Fatal("expected legacy meta merge")
	}
}

func TestMergeHostTurnNodesFromSkillGenerator_skipsNonDefaultTurn(t *testing.T) {
	host := &Workflow{SpecVersion: 2, ID: "other.turn", Nodes: map[string]Node{}}
	if err := MergeHostTurnNodesFromSkillGenerator(t.TempDir(), host); err != nil {
		t.Fatal(err)
	}
	if len(host.Nodes) != 0 {
		t.Fatalf("unexpected nodes %+v", host.Nodes)
	}
}
