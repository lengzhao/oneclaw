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
	if rtx.WorkflowNodeOutputs == nil || rtx.WorkflowNodeOutputs["adk_main"]["assistant_text"] != "hi" {
		t.Fatalf("got %#v", rtx.WorkflowNodeOutputs)
	}
	rtx.EmitNodeOutput(map[string]any{"user_prompt": "x"})
	if rtx.WorkflowNodeOutputs["adk_main"]["user_prompt"] != "x" {
		t.Fatalf("merge failed: %#v", rtx.WorkflowNodeOutputs["adk_main"])
	}
}
