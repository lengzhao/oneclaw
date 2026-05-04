package engine

import "strings"

// EmitNodeOutput merges payload into WorkflowNodeOutputs for the current graph node.
// Call only while wfexec has set CurrentNodeID for this handler; no-op on nil rtx, empty id, or empty payload.
func (rtx *RuntimeContext) EmitNodeOutput(payload map[string]any) {
	if rtx == nil || len(payload) == 0 {
		return
	}
	id := strings.TrimSpace(rtx.CurrentNodeID)
	if id == "" {
		return
	}
	if rtx.WorkflowNodeOutputs == nil {
		rtx.WorkflowNodeOutputs = make(map[string]map[string]any)
	}
	if rtx.WorkflowNodeOutputs[id] == nil {
		rtx.WorkflowNodeOutputs[id] = make(map[string]any)
	}
	for k, v := range payload {
		rtx.WorkflowNodeOutputs[id][k] = v
	}
}

// HasWorkflowNodeOutputStore reports whether WorkflowNodeOutputs was initialized (may still be empty).
func (rtx *RuntimeContext) HasWorkflowNodeOutputStore() bool {
	return rtx != nil && rtx.WorkflowNodeOutputs != nil
}

// WorkflowNodeOutputCopy returns a shallow copy of stored fields for nodeID, or nil if rtx/nodeID
// is invalid, the outputs map is nil, or the node has no rows yet.
func (rtx *RuntimeContext) WorkflowNodeOutputCopy(nodeID string) map[string]any {
	id := strings.TrimSpace(nodeID)
	if rtx == nil || id == "" || rtx.WorkflowNodeOutputs == nil {
		return nil
	}
	src := rtx.WorkflowNodeOutputs[id]
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
