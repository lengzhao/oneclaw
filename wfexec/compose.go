package wfexec

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
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
	addNativeIfBranches(w, state)
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

func addNativeIfBranches(w *compose.Workflow[TurnWorkflowInput, TurnWorkflowResult], state *compileState) {
	if w == nil || state == nil || state.wf == nil {
		return
	}
	for _, ifID := range sortedNodeIDs(state.wf) {
		ifNode, ok := state.wf.Nodes[ifID]
		if !ok || strings.TrimSpace(ifNode.Use) != "if" {
			continue
		}
		targets := ifTrueSuccessors(state.wf, ifID)
		if len(targets) == 0 {
			continue
		}
		endNodes := map[string]bool{compose.END: true}
		for _, t := range targets {
			endNodes[t] = true
		}
		w.AddBranch(ifID, compose.NewGraphMultiBranch(func(_ context.Context, in workflow.WorkflowNodeResult) (map[string]bool, error) {
			if ifResultPass(in) {
				out := make(map[string]bool, len(targets))
				for _, t := range targets {
					out[t] = true
				}
				return out, nil
			}
			return map[string]bool{compose.END: true}, nil
		}, endNodes))
	}
}

func ifTrueSuccessors(wf *workflow.Workflow, ifID string) []string {
	if wf == nil || strings.TrimSpace(ifID) == "" {
		return nil
	}
	seen := map[string]struct{}{}
	for nodeID, n := range wf.Nodes {
		if strings.TrimSpace(nodeID) == ifID {
			continue
		}
		for _, dep := range n.DependsOn {
			if strings.TrimSpace(dep) == ifID {
				seen[nodeID] = struct{}{}
			}
		}
		for _, dep := range inferredTemplateDeps(n) {
			if strings.TrimSpace(dep) == ifID {
				seen[nodeID] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// isIfBranchSuccessor reports whether consumer runs only on the if node's true branch.
// Edges from such consumers to the if node must not use a direct execution dependency:
// Eino requires WithNoDirectDependency for data mappings, and DependsOn-only edges must be omitted,
// otherwise execution bypasses AddBranch and the false branch never skips successors.
func isIfBranchSuccessor(wf *workflow.Workflow, fromIfID, consumerNodeID string) bool {
	if wf == nil {
		return false
	}
	fromIfID = strings.TrimSpace(fromIfID)
	consumerNodeID = strings.TrimSpace(consumerNodeID)
	if fromIfID == "" || consumerNodeID == "" {
		return false
	}
	n, ok := wf.Nodes[fromIfID]
	if !ok || strings.TrimSpace(n.Use) != "if" {
		return false
	}
	for _, t := range ifTrueSuccessors(wf, fromIfID) {
		if t == consumerNodeID {
			return true
		}
	}
	return false
}

func ifResultPass(in workflow.WorkflowNodeResult) bool {
	if in.Data != nil {
		if v, ok := in.Data["pass"]; ok {
			switch x := v.(type) {
			case bool:
				return x
			case string:
				return isTruthyString(x)
			}
		}
	}
	return isTruthyString(in.Text)
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
	if wf == nil {
		return nil
	}
	order, err := workflow.TopoSort(wf)
	if err == nil && len(order) == len(wf.Nodes) {
		return order
	}
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
	mapByDep := map[string][]*compose.FieldMapping{}
	simpleRefs := simpleTemplateNodeRefs(node)
	for _, dep := range simpleRefs {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		mapByDep[dep] = append(mapByDep[dep], compose.MapFields("Text", composeNodeTextInputField+"."+dep))
	}
	advancedRefs := advancedTemplateNodeRefs(node)
	advancedDepSet := map[string]struct{}{}
	for _, ref := range advancedRefs {
		dep := strings.TrimSpace(ref.NodeID)
		if dep == "" {
			continue
		}
		advancedDepSet[dep] = struct{}{}
	}
	for dep := range advancedDepSet {
		// Pass the whole Data object for this dependency; template renderer resolves nested keys dynamically.
		mapByDep[dep] = append(mapByDep[dep], compose.MapFields("Data", composeNodeDataInputField+"."+dep))
	}
	mapDeps := make([]string, 0, len(mapByDep))
	for dep := range mapByDep {
		mapDeps = append(mapDeps, dep)
	}
	sort.Strings(mapDeps)
	for _, dep := range mapDeps {
		mappings := mapByDep[dep]
		if isIfBranchSuccessor(state.wf, dep, nodeID) {
			n.AddInputWithOptions(dep, mappings, compose.WithNoDirectDependency())
			continue
		}
		n.AddInput(dep, mappings...)
	}
	simpleSet := map[string]struct{}{}
	for _, dep := range simpleRefs {
		if d := strings.TrimSpace(dep); d != "" {
			simpleSet[d] = struct{}{}
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
		if _, ok := advancedDepSet[dep]; ok {
			continue
		}
		if isIfBranchSuccessor(state.wf, dep, nodeID) {
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
	if req := strings.TrimSpace(paramString(node.Params, "require_truthy")); req != "" {
		ev := &EvalEnv{State: state, GraphInput: graphInput}
		rendered, err := ev.Render(req)
		if err != nil {
			return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: node %q (%s): require_truthy: %w", nodeID, node.Use, err)
		}
		if !isTruthyString(rendered) {
			slog.InfoContext(ctx, "wfexec.node.skip_require_truthy",
				"agent_type", agentType,
				"correlation_id", strings.TrimSpace(rtx.CorrelationID),
				"node", nodeID,
				"use", node.Use,
				"rendered", strings.TrimSpace(rendered),
			)
			return workflow.WorkflowNodeResult{}, nil
		}
	}
	in, err := nodeInputForExec(state, graphInput, node)
	if err != nil {
		return workflow.WorkflowNodeResult{}, err
	}
	out, err := h(ctx, in, NodeEnv{
		Runtime: rtx,
		NodeID:  nodeID,
		Node:    node,
		Eval:    &EvalEnv{State: state, GraphInput: graphInput},
	})
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
	if in == nil {
		return nil
	}
	switch m := in.(type) {
	case map[string]any:
		return m
	}
	v := reflect.ValueOf(in)
	for v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() != reflect.Map || v.Type().Key().Kind() != reflect.String {
		return nil
	}
	out := make(map[string]any, v.Len())
	it := v.MapRange()
	for it.Next() {
		out[it.Key().String()] = it.Value().Interface()
	}
	return out
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
