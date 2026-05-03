package wfexec

import (
	"context"
	"fmt"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

// Execute compiles wf to a compose Runnable via CompilePhase3Workflow (YAML DAG → Eino Graph; multi-sink uses _oneclaw_sink).
// Nodes with yaml async: true start their handler in a goroutine and the DAG proceeds without waiting; handler bodies still
// share rtx.ExecMu with synchronous nodes. Each call compiles fresh; cache the Runnable outside if reuse matters.
func Execute(ctx context.Context, wf *workflow.Workflow, reg *Registry, rtx *engine.RuntimeContext) error {
	if wf == nil || reg == nil || rtx == nil {
		return fmt.Errorf("wfexec: nil argument")
	}
	rtx.GoCtx = ctx
	run, err := CompilePhase3Workflow(ctx, wf, reg)
	if err != nil {
		return err
	}
	_, err = run.Invoke(ctx, rtx)
	return err
}
