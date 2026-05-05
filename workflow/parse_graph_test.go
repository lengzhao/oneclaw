package workflow

import "testing"

func TestParseBytes_explicitNodes(t *testing.T) {
	raw := []byte(`workflow_spec_version: 2
id: graph-explicit
nodes:
  root: { use: on_receive }
  tail:
    use: noop
    depends_on: [root]
`)
	w, err := ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Nodes) != 2 {
		t.Fatalf("%+v", w.Nodes)
	}
	if got := w.Nodes["tail"].DependsOn; len(got) != 1 || got[0] != "root" {
		t.Fatalf("tail depends_on: %+v", got)
	}
	if err := Validate(w); err != nil {
		t.Fatal(err)
	}
}
