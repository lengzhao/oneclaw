package wfexec

import (
	"context"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/workflow"
)

func init() {
	subagent.RegisterWorkflowExecutor(func(ctx context.Context, wf *workflow.Workflow, rtx *engine.RuntimeContext) error {
		reg := NewRegistry()
		if err := RegisterBuiltins(reg); err != nil {
			return err
		}
		return Execute(ctx, wf, reg, rtx)
	})
}
