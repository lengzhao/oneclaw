package workflow

import (
	"strings"
	"testing"
)

func TestComposeIndegree_dependsOnCount(t *testing.T) {
	w := &Workflow{
		SpecVersion: 2,
		ID:          "t",
		Nodes: map[string]Node{
			"a": {Use: "noop"},
			"b": {Use: "noop", DependsOn: []string{"a"}},
			"c": {Use: "noop", DependsOn: []string{"a", "b"}},
		},
	}
	if ComposeIndegree(w, "a") != 0 {
		t.Fatalf("a indegree want 0 got %d", ComposeIndegree(w, "a"))
	}
	if ComposeIndegree(w, "c") != 2 {
		t.Fatalf("c indegree want 2 got %d", ComposeIndegree(w, "c"))
	}
}

func TestSinkNodes_fork(t *testing.T) {
	w := &Workflow{
		SpecVersion: 2,
		ID:          "fork",
		Nodes: map[string]Node{
			"a": {Use: "noop"},
			"b": {Use: "noop", DependsOn: []string{"a"}},
			"c": {Use: "noop", DependsOn: []string{"a"}},
		},
	}
	s := SinkNodes(w)
	if len(s) != 2 || s[0] != "b" || s[1] != "c" {
		t.Fatalf("sinks %v", s)
	}
}

func TestValidateComposeFanOut_rejectsMixed(t *testing.T) {
	w := &Workflow{
		SpecVersion: 2,
		ID:          "mixed",
		Nodes: map[string]Node{
			"a": {Use: "noop"},
			"b": {Use: "noop", DependsOn: []string{"a"}},
			"c": {Use: "noop", DependsOn: []string{"a"}},
			"d": {Use: "noop", DependsOn: []string{"a", "c"}},
		},
	}
	err := ValidateComposeFanOut(w)
	if err == nil || !strings.Contains(err.Error(), "fans out") {
		t.Fatalf("expected fan-out error, got %v", err)
	}
}

func TestValidate_reservedOneclawNodeID(t *testing.T) {
	w := &Workflow{
		SpecVersion: 2,
		ID:          "x",
		Nodes:       map[string]Node{"_oneclaw_bad": {Use: "noop"}},
	}
	if err := Validate(w); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved prefix error, got %v", err)
	}
}
