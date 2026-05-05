package workflow

import (
	"strings"
)

// ReplyStreamEnabled reports whether outbound streaming is requested for this workflow document.
// True when any node with use "on_respond" or "llm" has params.stream truthy (after defaults merge).
func ReplyStreamEnabled(w *Workflow) bool {
	if w == nil {
		return false
	}
	for _, n := range w.Nodes {
		if n.Use != "on_respond" && n.Use != "llm" {
			continue
		}
		if paramsTruthy(n.Params, "stream") {
			return true
		}
	}
	return false
}

func paramsTruthy(params map[string]any, key string) bool {
	if len(params) == 0 {
		return false
	}
	v, ok := params[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "1" || s == "true" || s == "yes"
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	default:
		return false
	}
}
