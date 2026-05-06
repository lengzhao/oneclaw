package wfexec

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/engine"
)

func TestLookupRef_prefersMappedNodeTextInput(t *testing.T) {
	state := &compileState{}
	graphInput := map[string]any{
		composeNodeTextInputField: map[string]any{
			"intent": "from-mapped-input",
		},
	}

	got, err := lookupRef(state, graphInput, "$nodes.intent")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-mapped-input" {
		t.Fatalf("lookupRef($nodes.intent)=%q, want %q", got, "from-mapped-input")
	}
}

func TestLookupRef_prefersMappedNodeDataInput(t *testing.T) {
	state := &compileState{}
	graphInput := map[string]any{
		composeNodeDataInputField: map[string]any{
			"research": map[string]any{
				"documents": "from-mapped-input",
			},
		},
	}

	got, err := lookupRef(state, graphInput, "$nodes.research.documents")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-mapped-input" {
		t.Fatalf("lookupRef($nodes.research.documents)=%q, want %q", got, "from-mapped-input")
	}
}

func TestLookupRef_supportsNestedNodeDataPath(t *testing.T) {
	state := &compileState{}
	graphInput := map[string]any{
		composeNodeDataInputField: map[string]any{
			"research": map[string]any{
				"documents": map[string]any{
					"title": "from-mapped-input",
				},
			},
		},
	}

	got, err := lookupRef(state, graphInput, "$nodes.research.documents.title")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-mapped-input" {
		t.Fatalf("lookupRef($nodes.research.documents.title)=%q, want %q", got, "from-mapped-input")
	}
}

func TestLookupRef_runtimePostTurnCtx(t *testing.T) {
	tmp := t.TempDir()
	host := "default"
	corr := "c1"
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:     tmp,
			Turn:            engine.TurnContext{AgentID: host},
			CorrelationID:   corr,
			SessionSegment:  "seg",
			UserDataRoot:    filepath.Join(tmp, "ud"),
			InstructionRoot: filepath.Join(tmp, "instr"),
		},
	}
	state := &compileState{rtx: rtx}
	got, err := lookupRef(state, nil, "$runtime.post_turn.ctx")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" || !strings.Contains(got, "post_turn_ctx") {
		t.Fatalf("lookupRef($runtime.post_turn.ctx)=%q", got)
	}
}

func TestLookupRef_nodesRefDoesNotFallbackToStateResults(t *testing.T) {
	state := &compileState{}

	gotText, err := lookupRef(state, nil, "$nodes.intent")
	if err != nil {
		t.Fatal(err)
	}
	if gotText != "" {
		t.Fatalf("lookupRef($nodes.intent)=%q, want empty", gotText)
	}

	gotData, err := lookupRef(state, nil, "$nodes.intent.documents")
	if err != nil {
		t.Fatal(err)
	}
	if gotData != "" {
		t.Fatalf("lookupRef($nodes.intent.documents)=%q, want empty", gotData)
	}
}
