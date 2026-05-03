package wfexec

import (
	"context"
	"fmt"

	"github.com/lengzhao/oneclaw/workflow"
)

// Execute runs nodes in topological order (async flags ignored in phase 3 — nodes still run sequentially).
func Execute(ctx context.Context, wf *workflow.Workflow, reg *Registry, rtx *RuntimeContext) error {
	if wf == nil || reg == nil || rtx == nil {
		return fmt.Errorf("wfexec: nil argument")
	}
	rtx.GoCtx = ctx
	order, err := workflow.TopoSort(&wf.Graph)
	if err != nil {
		return err
	}
	for _, id := range order {
		node := wf.Graph.Nodes[id]
		h := reg.Lookup(node.Use)
		if h == nil {
			return fmt.Errorf("wfexec: no handler registered for use %q (node %q)", node.Use, id)
		}
		if err := h(rtx); err != nil {
			return fmt.Errorf("wfexec: node %q (%s): %w", id, node.Use, err)
		}
	}
	return nil
}
