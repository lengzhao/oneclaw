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
