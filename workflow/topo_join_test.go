package workflow

import "testing"

func TestTopoSort_diamond(t *testing.T) {
	w := &Workflow{
		SpecVersion: 2,
		ID:          "diamond",
		Nodes: map[string]Node{
			"a": {Use: "noop"},
			"b": {Use: "noop", DependsOn: []string{"a"}},
			"c": {Use: "noop", DependsOn: []string{"a"}},
			"d": {Use: "noop", DependsOn: []string{"b", "c"}},
		},
	}
	order, err := TopoSort(w)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 4 {
		t.Fatalf("topo len %d %v", len(order), order)
	}
	seen := map[string]bool{}
	for _, id := range order {
		seen[id] = true
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		if !seen[id] {
			t.Fatalf("missing %q in %v", id, order)
		}
	}
}
