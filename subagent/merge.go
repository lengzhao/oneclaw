package subagent

import (
	"os"
	"strings"
)

// SidechainMergeToolSuffix appends a short sidechain pointer to the run_agent / fork_context tool result.
func SidechainMergeToolSuffix() bool {
	v := strings.TrimSpace(os.Getenv("ONCLAW_SIDECCHAIN_MERGE"))
	if v == "" {
		return false
	}
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "tool") || strings.EqualFold(v, "append")
}

// SidechainMergeUserAfter appends a user-role message after tool outputs in the main transcript.
func SidechainMergeUserAfter() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("ONCLAW_SIDECCHAIN_MERGE")), "user")
}
