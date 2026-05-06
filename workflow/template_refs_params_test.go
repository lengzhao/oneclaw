package workflow

import "testing"

func TestTemplateReferencedNodes_scansParams(t *testing.T) {
	n := Node{
		Prompt: "$nodes.a.x",
		Input:  "",
		Params: map[string]any{
			"when_any": []any{
				map[string]any{"gt": []any{"$nodes.stats.distinct_tool_calls", 5}},
				"$nodes.other.flag",
			},
			"require_truthy": "$nodes.gate.pass",
		},
	}
	got := TemplateReferencedNodes(n)
	want := map[string]bool{"a": true, "stats": true, "other": true, "gate": true}
	for _, id := range got {
		if !want[id] {
			t.Fatalf("unexpected id %q", id)
		}
		delete(want, id)
	}
	if len(want) != 0 {
		t.Fatalf("missing ids: %v", want)
	}
}
