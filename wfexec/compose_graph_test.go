package wfexec

import (
	"context"
	"testing"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

func TestCompilePhase3Workflow_diamondJoin(t *testing.T) {
	ctx := context.Background()
	wf := &workflow.Workflow{
		SpecVersion: 1,
		ID:          "diamond",
		Graph: workflow.Graph{
			Entry: "a",
			Nodes: map[string]workflow.Node{
				"a": {Use: "on_receive"},
				"b": {Use: "noop"},
				"c": {Use: "noop"},
				"d": {Use: "noop"},
			},
			Edges: []workflow.Edge{
				{From: "a", To: "b"},
				{From: "a", To: "c"},
				{From: "b", To: "d"},
				{From: "c", To: "d"},
			},
		},
	}
	if err := workflow.Validate(wf); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	if err := RegisterPhase3Builtins(reg); err != nil {
		t.Fatal(err)
	}
	run, err := CompilePhase3Workflow(ctx, wf, reg)
	if err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{UserPrompt: "hi"}
	out, err := run.Invoke(ctx, rtx)
	if err != nil {
		t.Fatal(err)
	}
	if out != rtx {
		t.Fatal("expected same rtx pointer")
	}
}

func TestCompilePhase3Workflow_forkTwoSinks(t *testing.T) {
	ctx := context.Background()
	wf := &workflow.Workflow{
		SpecVersion: 1,
		ID:          "fork",
		Graph: workflow.Graph{
			Entry: "a",
			Nodes: map[string]workflow.Node{
				"a": {Use: "on_receive"},
				"b": {Use: "noop"},
				"c": {Use: "noop"},
			},
			Edges: []workflow.Edge{
				{From: "a", To: "b"},
				{From: "a", To: "c"},
			},
		},
	}
	if err := workflow.Validate(wf); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	if err := RegisterPhase3Builtins(reg); err != nil {
		t.Fatal(err)
	}
	run, err := CompilePhase3Workflow(ctx, wf, reg)
	if err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{UserPrompt: "hi"}
	out, err := run.Invoke(ctx, rtx)
	if err != nil {
		t.Fatal(err)
	}
	if out != rtx {
		t.Fatal("expected same rtx pointer")
	}
}

func TestCompilePhase3Workflow_linearStillWorks(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`workflow_spec_version: 1
id: linear
steps:
  - use: on_receive
  - use: noop
`)
	wf, err := workflow.ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.Validate(wf); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	if err := RegisterPhase3Builtins(reg); err != nil {
		t.Fatal(err)
	}
	run, err := CompilePhase3Workflow(ctx, wf, reg)
	if err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{UserPrompt: "x"}
	if _, err := run.Invoke(ctx, rtx); err != nil {
		t.Fatal(err)
	}
}
