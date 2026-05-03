package workflow

import (
	"strings"
	"testing"
)

func TestParseBytes_stepsExpandAndDefaults(t *testing.T) {
	raw := []byte(`workflow_spec_version: 1
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
	if w.Graph.Entry != "step_0" {
		t.Fatal(w.Graph.Entry)
	}
	if len(w.Graph.Edges) != 1 || w.Graph.Edges[0].From != "step_0" || w.Graph.Edges[0].To != "step_1" {
		t.Fatalf("%+v", w.Graph.Edges)
	}
	if w.Graph.Nodes["step_0"].Params["x"] != 1 {
		t.Fatalf("%+v", w.Graph.Nodes["step_0"].Params)
	}
	if err := Validate(w); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_rejectsCycle(t *testing.T) {
	w := &Workflow{
		SpecVersion: 1,
		ID:          "c",
		Graph: Graph{
			Entry: "a",
			Nodes: map[string]Node{
				"a": {Use: "noop"},
				"b": {Use: "noop"},
				"c": {Use: "noop"},
			},
			Edges: []Edge{{From: "a", To: "b"}, {From: "b", To: "c"}, {From: "c", To: "b"}},
		},
	}
	if err := Validate(w); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestValidate_reservedComposeNodeID(t *testing.T) {
	w := &Workflow{
		SpecVersion: 1,
		ID:          "bad",
		Graph: Graph{
			Entry: "start",
			Nodes: map[string]Node{"start": {Use: "noop"}},
			Edges: []Edge{},
		},
	}
	if err := Validate(w); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved id error, got %v", err)
	}
}

func TestValidate_agentRequiresAgentType(t *testing.T) {
	w := &Workflow{
		SpecVersion: 1,
		ID:          "x",
		Graph: Graph{
			Entry: "a",
			Nodes: map[string]Node{"a": {Use: "agent"}},
			Edges: []Edge{},
		},
	}
	if err := Validate(w); err == nil || !strings.Contains(err.Error(), "agent_type") {
		t.Fatalf("expected agent_type error, got %v", err)
	}
}

func TestValidate_postRespondAsyncAgents_defaultShape(t *testing.T) {
	raw := []byte(`workflow_spec_version: 1
id: default.turn
steps:
  - use: on_receive
  - use: noop
  - use: on_respond
  - id: memory_agent
    use: agent
    async: true
    params:
      agent_type: memory_extractor
  - id: skill_agent
    use: agent
    async: true
    params:
      agent_type: skill_generator
`)
	w, err := ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(w); err != nil {
		t.Fatal(err)
	}
	if !w.Graph.Nodes["memory_agent"].Async || !w.Graph.Nodes["skill_agent"].Async {
		t.Fatal("expected async branches")
	}
}

func TestValidate_unknownUse(t *testing.T) {
	w := &Workflow{
		SpecVersion: 1,
		ID:          "x",
		Graph: Graph{
			Entry: "m",
			Nodes: map[string]Node{"m": {Use: "unknown_node"}},
			Edges: []Edge{},
		},
	}
	if err := Validate(w); err == nil {
		t.Fatal("expected error")
	}
}
