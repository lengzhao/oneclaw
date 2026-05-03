package workflow

import (
	"fmt"
	"strings"
)

// AgentTypeParam returns trimmed params.agent_type for a workflow node.
func AgentTypeParam(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	v, ok := params["agent_type"]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}
