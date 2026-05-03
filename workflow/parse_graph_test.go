package workflow

import "testing"

func TestParseBytes_explicitGraph(t *testing.T) {
	raw := []byte(`workflow_spec_version: 1
id: graph-explicit
graph:
  entry: root
  nodes:
    root: { use: on_receive }
    tail: { use: noop }
  edges:
    - { from: root, to: tail }
`)
	w, err := ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if w.Graph.Entry != "root" {
		t.Fatal(w.Graph.Entry)
	}
	if len(w.Graph.Edges) != 1 {
		t.Fatal(w.Graph.Edges)
	}
	if err := Validate(w); err != nil {
		t.Fatal(err)
	}
}
