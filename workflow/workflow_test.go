package workflow

import (
	"strings"
	"testing"
)

func TestParseBytes_stepsExpandAndDefaults(t *testing.T) {
	raw := []byte(`workflow_spec_version: 2
id: t
defaults:
  x: 1
steps:
  - use: noop
  - use: noop
`)
	w, err := ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Nodes["step_1"].DependsOn; len(got) != 1 || got[0] != "step_0" {
		t.Fatalf("%+v", got)
	}
	if w.Nodes["step_0"].Params["x"] != 1 {
		t.Fatalf("%+v", w.Nodes["step_0"].Params)
	}
	if err := Validate(w); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_rejectsCycle(t *testing.T) {
	w := &Workflow{
		SpecVersion: 2,
		ID:          "c",
		Nodes: map[string]Node{
			"a": {Use: "noop"},
			"b": {Use: "noop", DependsOn: []string{"a", "c"}},
			"c": {Use: "noop", DependsOn: []string{"b"}},
		},
	}
	if err := Validate(w); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestValidate_reservedComposeNodeID(t *testing.T) {
	w := &Workflow{
		SpecVersion: 2,
		ID:          "bad",
		Nodes:       map[string]Node{"start": {Use: "noop"}},
	}
	if err := Validate(w); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved id error, got %v", err)
	}
}

func TestValidate_agentTaskRequiresAgentType(t *testing.T) {
	w := &Workflow{
		SpecVersion: 2,
		ID:          "x",
		Nodes:       map[string]Node{"a": {Use: "agent_task"}},
	}
	if err := Validate(w); err == nil || !strings.Contains(err.Error(), "agent_type") {
		t.Fatalf("expected agent_type error, got %v", err)
	}
}

func TestValidate_postRespondAsyncAgents_defaultShape(t *testing.T) {
	raw := []byte(`workflow_spec_version: 2
id: default.turn
steps:
  - use: on_receive
  - use: noop
  - use: on_respond
  - id: memory_agent
    use: agent_task
    async: true
    agent_type: memory_extractor
  - id: skill_agent
    use: agent_task
    async: true
    agent_type: skill_generator
`)
	w, err := ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(w); err != nil {
		t.Fatal(err)
	}
	if !w.Nodes["memory_agent"].Async || !w.Nodes["skill_agent"].Async {
		t.Fatal("expected async branches")
	}
}

func TestValidate_unknownUse(t *testing.T) {
	w := &Workflow{
		SpecVersion: 2,
		ID:          "x",
		Nodes:       map[string]Node{"m": {Use: "unknown_node"}},
	}
	if err := Validate(w); err == nil {
		t.Fatal("expected error")
	}
}
