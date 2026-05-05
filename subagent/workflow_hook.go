package subagent

import (
	"context"
	"fmt"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

var workflowExecute func(ctx context.Context, wf *workflow.Workflow, rtx *engine.RuntimeContext) error

// RegisterWorkflowExecutor wires YAML workflow execution for ExecuteSubAgentTurn.
// github.com/lengzhao/oneclaw/wfexec registers this from init; import _ wfexec if you spawn sub-agents without importing wfexec otherwise.
func RegisterWorkflowExecutor(fn func(ctx context.Context, wf *workflow.Workflow, rtx *engine.RuntimeContext) error) {
	workflowExecute = fn
}

func runWorkflow(ctx context.Context, wf *workflow.Workflow, rtx *engine.RuntimeContext) error {
	if workflowExecute == nil {
		return fmt.Errorf("subagent: workflow executor not registered (import github.com/lengzhao/oneclaw/wfexec)")
	}
	return workflowExecute(ctx, wf, rtx)
}
