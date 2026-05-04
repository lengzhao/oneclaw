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
