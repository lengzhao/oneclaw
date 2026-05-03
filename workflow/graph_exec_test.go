package workflow

import (
	"strings"
	"testing"
)

func TestComposeIndegree_entryGetsStart(t *testing.T) {
	g := &Graph{
		Entry: "a",
		Nodes: map[string]Node{"a": {Use: "noop"}, "b": {Use: "noop"}},
		Edges: []Edge{{From: "a", To: "b"}},
	}
	if ComposeIndegree(g, "a") != 1 {
		t.Fatalf("entry compose indegree want 1 got %d", ComposeIndegree(g, "a"))
	}
	if ComposeIndegree(g, "b") != 1 {
		t.Fatalf("b compose indegree want 1 got %d", ComposeIndegree(g, "b"))
	}
}

func TestComposeIndegree_join(t *testing.T) {
	g := &Graph{
		Entry: "a",
		Nodes: map[string]Node{
			"a": {Use: "noop"},
			"b": {Use: "noop"},
			"c": {Use: "noop"},
			"d": {Use: "noop"},
		},
		Edges: []Edge{
			{From: "a", To: "b"},
			{From: "a", To: "c"},
			{From: "b", To: "d"},
			{From: "c", To: "d"},
		},
	}
	if ComposeIndegree(g, "d") != 2 {
		t.Fatalf("join indegree want 2 got %d", ComposeIndegree(g, "d"))
	}
}

func TestSinkNodes_fork(t *testing.T) {
	g := &Graph{
		Entry: "a",
		Nodes: map[string]Node{
			"a": {Use: "noop"},
			"b": {Use: "noop"},
			"c": {Use: "noop"},
		},
		Edges: []Edge{{From: "a", To: "b"}, {From: "a", To: "c"}},
	}
	s := SinkNodes(g)
	if len(s) != 2 || s[0] != "b" || s[1] != "c" {
		t.Fatalf("sinks %v", s)
	}
}

func TestValidateComposeFanOut_rejectsMixed(t *testing.T) {
	g := &Graph{
		Entry: "a",
		Nodes: map[string]Node{
			"a": {Use: "noop"},
			"b": {Use: "noop"},
			"c": {Use: "noop"},
			"d": {Use: "noop"},
		},
		Edges: []Edge{
			{From: "a", To: "b"},
			{From: "a", To: "c"},
			{From: "a", To: "d"},
			{From: "c", To: "d"},
		},
	}
	err := ValidateComposeFanOut(g)
	if err == nil || !strings.Contains(err.Error(), "fans out") {
		t.Fatalf("expected fan-out error, got %v", err)
	}
}

func TestValidate_reservedOneclawNodeID(t *testing.T) {
	w := &Workflow{
		SpecVersion: 1,
		ID:          "x",
		Graph: Graph{
			Entry: "_oneclaw_bad",
			Nodes: map[string]Node{"_oneclaw_bad": {Use: "noop"}},
			Edges: []Edge{},
		},
	}
	if err := Validate(w); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved prefix error, got %v", err)
	}
}
