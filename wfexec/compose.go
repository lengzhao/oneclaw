package wfexec

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/cloudwego/eino/compose"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

// composeSinkMergeID terminates multi-sink DAGs so END receives a single merge edge (Eino limitation).
const composeSinkMergeID = "_oneclaw_sink"

// CompilePhase3Workflow builds an Eino compose Runnable from the workflow YAML graph:
// real START / edges / END (plus a synthetic sink merge when there are multiple sinks).
// Nodes with async: true spawn a goroutine for the handler; the compose layer treats the node as
// succeeded immediately so downstream nodes can run without waiting (see engine.AsyncHandlerFinished).
func CompilePhase3Workflow(ctx context.Context, wf *workflow.Workflow, reg *Registry) (compose.Runnable[*engine.RuntimeContext, *engine.RuntimeContext], error) {
	if wf == nil || reg == nil {
		return nil, fmt.Errorf("wfexec: nil workflow or registry")
	}
	if len(wf.Graph.Nodes) == 0 {
		return nil, fmt.Errorf("wfexec: empty workflow graph")
	}

	nodeIDs := sortedNodeIDs(&wf.Graph)
	needsKey := outputKeySenders(&wf.Graph)

	sinks := workflow.SinkNodes(&wf.Graph)
	if len(sinks) > 1 {
		for _, s := range sinks {
			needsKey[s] = true
		}
	}

	g := compose.NewGraph[*engine.RuntimeContext, *engine.RuntimeContext]()
	for _, id := range nodeIDs {
		if err := addWorkflowLambda(g, id, wf, reg, needsKey); err != nil {
			return nil, err
		}
	}

	if len(sinks) > 1 {
		if err := g.AddLambdaNode(composeSinkMergeID, compose.InvokableLambda(func(_ context.Context, in map[string]any) (*engine.RuntimeContext, error) {
			return coalesceRTX(in)
		})); err != nil {
			return nil, fmt.Errorf("wfexec: add sink merge: %w", err)
		}
	}

	if err := g.AddEdge(compose.START, wf.Graph.Entry); err != nil {
		return nil, fmt.Errorf("wfexec: START edge: %w", err)
	}
	for _, e := range wf.Graph.Edges {
		if err := g.AddEdge(e.From, e.To); err != nil {
			return nil, fmt.Errorf("wfexec: edge %q -> %q: %w", e.From, e.To, err)
		}
	}

	switch len(sinks) {
	case 1:
		if err := g.AddEdge(sinks[0], compose.END); err != nil {
			return nil, fmt.Errorf("wfexec: END edge: %w", err)
		}
	default:
		for _, s := range sinks {
			if err := g.AddEdge(s, composeSinkMergeID); err != nil {
				return nil, fmt.Errorf("wfexec: sink edge %q: %w", s, err)
			}
		}
		if err := g.AddEdge(composeSinkMergeID, compose.END); err != nil {
			return nil, fmt.Errorf("wfexec: sink merge -> END: %w", err)
		}
	}

	run, err := g.Compile(ctx, compose.WithNodeTriggerMode(compose.AllPredecessor))
	if err != nil {
		return nil, fmt.Errorf("wfexec: compile compose graph: %w", err)
	}
	return run, nil
}

func sortedNodeIDs(g *workflow.Graph) []string {
	ids := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func outputKeySenders(g *workflow.Graph) map[string]bool {
	out := map[string]bool{}
	for _, e := range g.Edges {
		if workflow.ComposeIndegree(g, e.To) >= 2 {
			out[e.From] = true
		}
	}
	return out
}

func addWorkflowLambda(
	g *compose.Graph[*engine.RuntimeContext, *engine.RuntimeContext],
	nodeID string,
	wf *workflow.Workflow,
	reg *Registry,
	needsKey map[string]bool,
) error {
	node := wf.Graph.Nodes[nodeID]
	use := node.Use
	indeg := workflow.ComposeIndegree(&wf.Graph, nodeID)

	var opts []compose.GraphAddNodeOpt
	if needsKey[nodeID] {
		opts = append(opts, compose.WithOutputKey(nodeID))
	}

	if indeg >= 2 {
		err := g.AddLambdaNode(nodeID, compose.InvokableLambda(func(ctx context.Context, in map[string]any) (*engine.RuntimeContext, error) {
			rtx, err := coalesceRTX(in)
			if err != nil {
				return nil, fmt.Errorf("wfexec: node %q: %w", nodeID, err)
			}
			return invokeWorkflowNode(ctx, rtx, nodeID, node, use, reg)
		}), opts...)
		if err != nil {
			return fmt.Errorf("wfexec: add node %q: %w", nodeID, err)
		}
		return nil
	}

	err := g.AddLambdaNode(nodeID, compose.InvokableLambda(func(ctx context.Context, rtx *engine.RuntimeContext) (*engine.RuntimeContext, error) {
		if rtx == nil {
			return nil, fmt.Errorf("wfexec: node %q: nil runtime context", nodeID)
		}
		return invokeWorkflowNode(ctx, rtx, nodeID, node, use, reg)
	}), opts...)
	if err != nil {
		return fmt.Errorf("wfexec: add node %q: %w", nodeID, err)
	}
	return nil
}

func invokeWorkflowNode(
	ctx context.Context,
	rtx *engine.RuntimeContext,
	nodeID string,
	node workflow.Node,
	use string,
	reg *Registry,
) (*engine.RuntimeContext, error) {
	if node.Async {
		rtx.ExecMu.Lock()
		snap := engine.CaptureReadSnapshot(rtx)
		rtx.ExecMu.Unlock()
		bgCtx := engine.WithReadSnapshot(context.WithoutCancel(ctx), snap)
		go runAsyncWorkflowHandler(bgCtx, rtx, nodeID, node, use, reg)
		return rtx, nil
	}
	if err := executeWorkflowHandler(ctx, rtx, nodeID, node, use, reg); err != nil {
		return nil, err
	}
	return rtx, nil
}

func runAsyncWorkflowHandler(bgCtx context.Context, rtx *engine.RuntimeContext, nodeID string, node workflow.Node, use string, reg *Registry) {
	var handlerErr error
	defer func() {
		if r := recover(); r != nil {
			handlerErr = fmt.Errorf("panic: %v", r)
			slog.Error("wfexec: async workflow node panic", "node", nodeID, "use", use, "recover", r)
		}
		rtx.RecordAsyncHandlerEnd(nodeID, handlerErr)
	}()
	handlerErr = executeWorkflowHandler(bgCtx, rtx, nodeID, node, use, reg)
	if handlerErr != nil {
		slog.Error("wfexec: async workflow node failed", "node", nodeID, "use", use, "err", handlerErr)
	}
}

func executeWorkflowHandler(
	ctx context.Context,
	rtx *engine.RuntimeContext,
	nodeID string,
	node workflow.Node,
	use string,
	reg *Registry,
) error {
	rtx.ExecMu.Lock()
	defer rtx.ExecMu.Unlock()

	rtx.GoCtx = ctx
	rtx.CurrentNodeID = nodeID
	rtx.CurrentAsync = node.Async
	if len(node.Params) > 0 {
		rtx.CurrentParams = cloneParams(node.Params)
	} else {
		rtx.CurrentParams = nil
	}
	defer func() {
		rtx.CurrentNodeID = ""
		rtx.CurrentAsync = false
		rtx.CurrentParams = nil
	}()

	h := reg.Lookup(use)
	if h == nil {
		return fmt.Errorf("wfexec: no handler registered for use %q (node %q)", use, nodeID)
	}
	if err := h(rtx); err != nil {
		return fmt.Errorf("wfexec: node %q (%s): %w", nodeID, use, err)
	}
	return nil
}

func cloneParams(p map[string]any) map[string]any {
	if len(p) == 0 {
		return nil
	}
	out := make(map[string]any, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}
