package workflow

import "testing"

func TestReplyStreamEnabled(t *testing.T) {
	raw := []byte(`workflow_spec_version: 2
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

func TestReplyStreamEnabled_llmParam(t *testing.T) {
	w := &Workflow{
		SpecVersion: 2,
		ID:          "x",
		Nodes: map[string]Node{
			"a": {Use: "noop"},
			"b": {Use: "llm", Params: map[string]any{"stream": true}, DependsOn: []string{"a"}},
		},
	}
	if err := Validate(w); err != nil {
		t.Fatal(err)
	}
	if !ReplyStreamEnabled(w) {
		t.Fatal("expected stream from llm")
	}
}

func TestReplyStreamEnabled_offByDefault(t *testing.T) {
	raw := []byte(`workflow_spec_version: 2
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
