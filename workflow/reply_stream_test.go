package workflow

import "testing"

func TestReplyStreamEnabled(t *testing.T) {
	raw := []byte(`workflow_spec_version: 1
id: t
steps:
  - use: noop
  - id: on_respond
    use: on_respond
    params:
      stream: true
`)
	w, err := ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !ReplyStreamEnabled(w) {
		t.Fatal("expected stream enabled")
	}
}

func TestReplyStreamEnabled_adkMainParam(t *testing.T) {
	w := &Workflow{
		SpecVersion: 1,
		ID:          "x",
		Graph: Graph{
			Entry: "a",
			Nodes: map[string]Node{
				"a": {Use: "noop"},
				"b": {Use: "adk_main", Params: map[string]any{"stream": true}},
			},
			Edges: []Edge{{From: "a", To: "b"}},
		},
	}
	if err := Validate(w); err != nil {
		t.Fatal(err)
	}
	if !ReplyStreamEnabled(w) {
		t.Fatal("expected stream from adk_main")
	}
}

func TestReplyStreamEnabled_offByDefault(t *testing.T) {
	raw := []byte(`workflow_spec_version: 1
id: t
steps:
  - use: noop
  - use: on_respond
`)
	w, err := ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ReplyStreamEnabled(w) {
		t.Fatal("expected stream disabled")
	}
}
