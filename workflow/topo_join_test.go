package workflow

import "testing"

func TestTopoSort_diamond(t *testing.T) {
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
	order, err := TopoSort(g)
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
