package subagent

import (
	"github.com/lengzhao/oneclaw/toolhost"
	"github.com/lengzhao/oneclaw/tools"
)

// DefaultSubagentToolTemplate is the PicoClaw-style narrow default when a catalog sub-agent
// omits `tools:` — not a full inheritance of the parent registry (copy of tools.DefaultSubagentBuiltinIDs).
var DefaultSubagentToolTemplate = append([]string(nil), tools.DefaultSubagentBuiltinIDs...)

func intersectParentTemplate(parent toolhost.Registry, template []string) []string {
	if parent == nil || len(template) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	for _, n := range parent.Names() {
		seen[n] = true
	}
	var out []string
	for _, want := range template {
		if want == "" || !seen[want] {
			continue
		}
		out = append(out, want)
	}
	return out
}
