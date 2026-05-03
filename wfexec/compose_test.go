package wfexec

import (
	"context"
	"testing"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

func TestCompilePhase3Workflow_invokeMinimal(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`workflow_spec_version: 1
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
		t.Fatal("expected same runtime pointer pass-through")
	}
}
