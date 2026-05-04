package subagent

import (
	"context"
	"fmt"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

var phase3WorkflowExecute func(ctx context.Context, wf *workflow.Workflow, rtx *engine.RuntimeContext) error

// RegisterPhase3WorkflowExecutor wires YAML DAG execution for ExecuteSubAgentTurn.
// github.com/lengzhao/oneclaw/wfexec registers this from init; import _ wfexec if you spawn sub-agents without importing wfexec otherwise.
func RegisterPhase3WorkflowExecutor(fn func(ctx context.Context, wf *workflow.Workflow, rtx *engine.RuntimeContext) error) {
	phase3WorkflowExecute = fn
}

func runPhase3Workflow(ctx context.Context, wf *workflow.Workflow, rtx *engine.RuntimeContext) error {
	if phase3WorkflowExecute == nil {
		return fmt.Errorf("subagent: phase-3 workflow executor not registered (import github.com/lengzhao/oneclaw/wfexec)")
	}
	return phase3WorkflowExecute(ctx, wf, rtx)
}
