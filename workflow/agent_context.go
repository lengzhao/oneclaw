package workflow

import (
	"fmt"
	"strings"
)

// Agent context attachment refs (params.context[] on use: agent_task nodes).
const (
	AgentContextRefRunJournal    = "run_journal"
	AgentContextRefWorkflowNode  = "workflow_node"
	AgentContextRefTranscript    = "transcript"
	AgentContextAsUserMessage    = "user_message"
	AgentContextAsToolBinding    = "tool_binding_only"
	AgentContextAsPathMetadata   = "path_metadata"
	AgentContextScopeCurrentTurn = "current_turn"
	AgentContextScopeFull        = "full"
)

// AgentContextAttachment is one element of params.context for sub-agent prompt assembly.
type AgentContextAttachment struct {
	Ref    string
	Scope  string
	As     string
	Label  string
	Select map[string]any
}

// ParseAgentContextAttachments parses params.context for use: agent_task nodes.
func ParseAgentContextAttachments(params map[string]any) []AgentContextAttachment {
	var out []AgentContextAttachment
	if len(params) == 0 {
		return nil
	}
	if raw, ok := params["context"]; ok && raw != nil {
		out = append(out, parseContextSlice(raw)...)
	}
	return out
}

func parseContextSlice(raw any) []AgentContextAttachment {
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return nil
	}
	var out []AgentContextAttachment
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ref := strings.TrimSpace(mapStr(m, "ref"))
		if ref == "" {
			continue
		}
		a := AgentContextAttachment{
			Ref:    ref,
			Scope:  strings.TrimSpace(mapStr(m, "scope")),
			As:     strings.TrimSpace(mapStr(m, "as")),
			Label:  strings.TrimSpace(mapStr(m, "label")),
			Select: nil,
		}
		if sel, ok := m["select"].(map[string]any); ok && len(sel) > 0 {
			cp := make(map[string]any, len(sel))
			for k, v := range sel {
				cp[k] = v
			}
			a.Select = cp
		}
		if a.Scope == "" && ref == AgentContextRefRunJournal {
			a.Scope = AgentContextScopeCurrentTurn
		}
		if a.As == "" {
			a.As = AgentContextAsUserMessage
		}
		out = append(out, a)
	}
	return out
}

func mapStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
