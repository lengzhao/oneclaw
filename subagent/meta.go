package subagent

// IsMetaToolName identifies meta-tools not listed on the builtins seed registry (e.g. run_agent).
func IsMetaToolName(name string) bool {
	return name == "run_agent"
}
