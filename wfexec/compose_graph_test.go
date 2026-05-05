package wfexec

import (
	"context"
	"testing"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

func TestCompileEinoWorkflow_diamondJoin(t *testing.T) {
	ctx := context.Background()
	wf := &workflow.Workflow{
		SpecVersion: 2,
		ID:          "diamond",
		Nodes: map[string]workflow.Node{
			"a": {Use: "on_receive"},
			"b": {Use: "noop", DependsOn: []string{"a"}},
			"c": {Use: "noop", DependsOn: []string{"a"}},
			"d": {Use: "noop", DependsOn: []string{"b", "c"}},
		},
		End: "d",
	}
	if err := workflow.Validate(wf); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "hi"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}
	out, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "hi", Runtime: rtx})
	if err != nil {
		t.Fatal(err)
	}
	if out.Runtime != rtx {
		t.Fatal("expected same rtx pointer")
	}
}

func TestCompileEinoWorkflow_forkTwoSinks(t *testing.T) {
	ctx := context.Background()
	wf := &workflow.Workflow{
		SpecVersion: 2,
		ID:          "fork",
		Nodes: map[string]workflow.Node{
			"a": {Use: "on_receive"},
			"b": {Use: "noop", DependsOn: []string{"a"}},
			"c": {Use: "noop", DependsOn: []string{"a"}},
		},
		End: "b",
	}
	if err := workflow.Validate(wf); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "hi"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}
	out, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "hi", Runtime: rtx})
	if err != nil {
		t.Fatal(err)
	}
	if out.Runtime != rtx {
		t.Fatal("expected same rtx pointer")
	}
}

func TestCompileEinoWorkflow_linearStillWorks(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`workflow_spec_version: 2
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
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "x"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "x", Runtime: rtx}); err != nil {
		t.Fatal(err)
	}
}
