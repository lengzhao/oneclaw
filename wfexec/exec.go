package wfexec

import (
	"context"
	"fmt"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

// Execute compiles and runs workflow v2 via Eino compose.Workflow.
func Execute(ctx context.Context, wf *workflow.Workflow, reg *Registry, rtx *engine.RuntimeContext) error {
	if wf == nil || reg == nil || rtx == nil {
		return fmt.Errorf("wfexec: nil argument")
	}
	rtx.GoCtx = ctx
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		return err
	}
	_, err = run.Invoke(ctx, TurnWorkflowInput{
		UserPrompt: rtx.EffectiveUserPrompt(),
		AgentID:    rtx.Turn.AgentID,
		Runtime:    rtx,
	})
	return err
}
