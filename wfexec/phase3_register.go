package wfexec

import (
	"context"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/workflow"
)

func init() {
	subagent.RegisterPhase3WorkflowExecutor(func(ctx context.Context, wf *workflow.Workflow, rtx *engine.RuntimeContext) error {
		reg := NewRegistry()
		if err := RegisterPhase3Builtins(reg); err != nil {
			return err
		}
		return Execute(ctx, wf, reg, rtx)
	})
}
