package wfexec

import (
	"context"
	"testing"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

func TestCompileEinoWorkflow_invokeMinimal(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`workflow_spec_version: 2
id: t
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
		t.Fatal("expected same runtime pointer pass-through")
	}
}
