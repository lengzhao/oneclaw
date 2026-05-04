package engine

import "testing"

func TestRuntimeContext_EmitNodeOutput(t *testing.T) {
	var rtx RuntimeContext
	rtx.EmitNodeOutput(map[string]any{"a": 1})
	if rtx.WorkflowNodeOutputs != nil {
		t.Fatal("expected no writes without CurrentNodeID")
	}

	rtx.CurrentNodeID = "adk_main"
	rtx.EmitNodeOutput(map[string]any{
		"use":            "adk_main",
		"assistant_text": "hi",
	})
	got := rtx.WorkflowNodeOutputCopy("adk_main")
	if got == nil || got["assistant_text"] != "hi" {
		t.Fatalf("got %#v", got)
	}
	rtx.EmitNodeOutput(map[string]any{"user_prompt": "x"})
	got = rtx.WorkflowNodeOutputCopy("adk_main")
	if got == nil || got["user_prompt"] != "x" {
		t.Fatalf("merge failed: %#v", got)
	}
}

func TestRuntimeContext_WorkflowNodeOutputCopy_isolation(t *testing.T) {
	var rtx RuntimeContext
	rtx.WorkflowNodeOutputs = map[string]map[string]any{
		"n": {"k": "v"},
	}
	cp := rtx.WorkflowNodeOutputCopy("n")
	if cp == nil || cp["k"] != "v" {
		t.Fatal(cp)
	}
	cp["k"] = "mutated"
	if rtx.WorkflowNodeOutputs["n"]["k"] != "v" {
		t.Fatal("copy must not alias inner values for scalar reassignment path")
	}
}
