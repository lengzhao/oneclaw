package wfexec

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

const composeResultNodeID = "_oneclaw_result"

func CompileEinoWorkflow(ctx context.Context, wf *workflow.Workflow, reg *Registry, rtx *engine.RuntimeContext) (compose.Runnable[TurnWorkflowInput, TurnWorkflowResult], error) {
	if wf == nil || reg == nil || rtx == nil {
		return nil, fmt.Errorf("wfexec: nil argument")
	}
	if len(wf.Nodes) == 0 {
		return nil, fmt.Errorf("wfexec: empty workflow nodes")
	}

	state := &compileState{
		wf:  wf,
		reg: reg,
		rtx: rtx,
	}
	w := compose.NewWorkflow[TurnWorkflowInput, TurnWorkflowResult]()
	for _, id := range sortedNodeIDs(wf) {
		if err := addWorkflowNodeV2(w, state, id); err != nil {
			return nil, err
		}
	}
	resultNode := w.AddLambdaNode(composeResultNodeID, compose.InvokableLambda(func(_ context.Context, _ any) (TurnWorkflowResult, error) {
		return TurnWorkflowResult{
			Assistant: strings.TrimSpace(state.rtx.Assistant),
			Runtime:   state.rtx,
		}, nil
	}))
	for _, dep := range state.finalDependsOn() {
		resultNode.AddDependency(dep)
	}
	w.End().AddInput(composeResultNodeID)

	run, err := w.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("wfexec: compile compose workflow: %w", err)
	}
	return run, nil
}

type compileState struct {
	wf  *workflow.Workflow
	reg *Registry
	rtx *engine.RuntimeContext
}

func (s *compileState) finalDependsOn() []string {
	if end := strings.TrimSpace(s.wf.End); end != "" {
		return []string{end}
	}
	order, err := workflow.TopoSort(s.wf)
	if err == nil {
		for i := len(order) - 1; i >= 0; i-- {
			if !s.wf.Nodes[order[i]].Async {
				return []string{order[i]}
			}
		}
	}
	return workflow.SinkNodes(s.wf)
}

func sortedNodeIDs(wf *workflow.Workflow) []string {
	ids := make([]string, 0, len(wf.Nodes))
	for id := range wf.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func addWorkflowNodeV2(w *compose.Workflow[TurnWorkflowInput, TurnWorkflowResult], state *compileState, nodeID string) error {
	node := state.wf.Nodes[nodeID]
	n := w.AddLambdaNode(nodeID, compose.InvokableLambda(func(ctx context.Context, in any) (workflow.WorkflowNodeResult, error) {
		return invokeWorkflowNodeV2(ctx, state, nodeID, node, in)
	}))
	n.AddInput(compose.START, compose.MapFields("Runtime", "runtime"))
	simpleRefs := simpleTemplateNodeRefs(node)
	for _, dep := range simpleRefs {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		n.AddInput(dep, compose.MapFields("Text", composeNodeTextInputField+"."+dep))
	}
	advancedRefs := advancedTemplateNodeRefs(node)
	for _, ref := range advancedRefs {
		dep := strings.TrimSpace(ref.NodeID)
		if dep == "" || len(ref.FieldPath) == 0 {
			continue
		}
		fromPath := "Data." + strings.Join(ref.FieldPath, ".")
		toPath := composeNodeDataInputField + "." + dep + "." + strings.Join(ref.FieldPath, ".")
		n.AddInput(dep, compose.MapFields(fromPath, toPath))
	}
	simpleSet := map[string]struct{}{}
	for _, dep := range simpleRefs {
		simpleSet[dep] = struct{}{}
	}
	advancedSet := map[string]struct{}{}
	for _, ref := range advancedRefs {
		if strings.TrimSpace(ref.NodeID) != "" {
			advancedSet[ref.NodeID] = struct{}{}
		}
	}
	deps := inferredTemplateDeps(node)
	for _, dep := range deps {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if _, ok := simpleSet[dep]; ok {
			continue
		}
		if _, ok := advancedSet[dep]; ok {
			continue
		}
		n.AddDependency(dep)
	}
	return nil
}

func invokeWorkflowNodeV2(ctx context.Context, state *compileState, nodeID string, node workflow.Node, graphInputAny any) (workflow.WorkflowNodeResult, error) {
	graphInput := asGraphInputMap(graphInputAny)
	if node.Async {
		state.rtx.ExecMu.Lock()
		snap := engine.CaptureReadSnapshot(state.rtx)
		state.rtx.ExecMu.Unlock()
		bgCtx := engine.WithReadSnapshot(context.WithoutCancel(ctx), snap)
		go runAsyncWorkflowHandlerV2(bgCtx, state, nodeID, node, graphInput)
		return workflow.WorkflowNodeResult{}, nil
	}
	out, err := executeWorkflowNodeV2(ctx, state, nodeID, node, graphInput)
	if err != nil {
		return workflow.WorkflowNodeResult{}, err
	}
	return out, nil
}

func runAsyncWorkflowHandlerV2(bgCtx context.Context, state *compileState, nodeID string, node workflow.Node, graphInput map[string]any) {
	var handlerErr error
	defer func() {
		if r := recover(); r != nil {
			handlerErr = fmt.Errorf("panic: %v", r)
			slog.Error("wfexec: async workflow node panic", "node", nodeID, "use", node.Use, "recover", r)
		}
		state.rtx.RecordAsyncHandlerEnd(nodeID, handlerErr)
	}()
	_, err := executeWorkflowNodeV2(bgCtx, state, nodeID, node, graphInput)
	handlerErr = err
	if err == nil {
		return
	}
	slog.Error("wfexec: async workflow node failed", "node", nodeID, "use", node.Use, "err", err)
}

func executeWorkflowNodeV2(ctx context.Context, state *compileState, nodeID string, node workflow.Node, graphInput map[string]any) (workflow.WorkflowNodeResult, error) {
	rtx := state.rtx
	rtx.ExecMu.Lock()
	defer rtx.ExecMu.Unlock()

	rtx.GoCtx = ctx
	rtx.CurrentNodeID = nodeID
	rtx.CurrentAsync = node.Async
	rtx.CurrentParams = cloneParams(node.Params)
	defer func() {
		rtx.CurrentNodeID = ""
		rtx.CurrentAsync = false
		rtx.CurrentParams = nil
	}()
	agentType := strings.TrimSpace(rtx.Turn.AgentID)
	if rtx.Agent != nil && strings.TrimSpace(rtx.Agent.AgentType) != "" {
		agentType = strings.TrimSpace(rtx.Agent.AgentType)
	}
	start := time.Now()
	slog.InfoContext(ctx, "wfexec.node.start",
		"agent_type", agentType,
		"correlation_id", strings.TrimSpace(rtx.CorrelationID),
		"node", nodeID,
		"use", node.Use,
		"async", node.Async,
	)

	h := state.reg.Lookup(node.Use)
	if h == nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: no handler registered for use %q (node %q)", node.Use, nodeID)
	}
	in, err := nodeInputForExec(state, graphInput, node)
	if err != nil {
		return workflow.WorkflowNodeResult{}, err
	}
	out, err := h(ctx, in, NodeEnv{Runtime: rtx, NodeID: nodeID, Node: node})
	if err != nil {
		slog.ErrorContext(ctx, "wfexec.node.failed",
			"agent_type", agentType,
			"correlation_id", strings.TrimSpace(rtx.CorrelationID),
			"node", nodeID,
			"use", node.Use,
			"async", node.Async,
			"elapsed_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: node %q (%s): %w", nodeID, node.Use, err)
	}
	if out.Data == nil {
		out.Data = map[string]any{}
	}
	slog.InfoContext(ctx, "wfexec.node.done",
		"agent_type", agentType,
		"correlation_id", strings.TrimSpace(rtx.CorrelationID),
		"node", nodeID,
		"use", node.Use,
		"async", node.Async,
		"elapsed_ms", time.Since(start).Milliseconds(),
	)
	return out, nil
}

func asGraphInputMap(in any) map[string]any {
	switch m := in.(type) {
	case nil:
		return nil
	case map[string]any:
		return m
	default:
		return nil
	}
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
